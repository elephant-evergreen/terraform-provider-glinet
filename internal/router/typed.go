package router

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// OwnerTokenOption records which Terraform state manages the section.
const OwnerTokenOption = "terraform_owner_id"

// ManagedOption marks a UCI section as owned by a typed Terraform resource.
// It is written in the same atomic transaction as the section itself, so the
// safety checks can distinguish managed sections from firmware-managed ones
// without a separate registry.
const ManagedOption = "terraform_owner"

func IsManaged(options map[string]string) bool {
	return options[ManagedOption] == "1"
}

func ManagedOwner(options map[string]string) string {
	return options[OwnerTokenOption]
}

func SetManagedOwner(options map[string]string, token string) {
	options[ManagedOption] = "1"
	options[OwnerTokenOption] = token
}

func MarkManaged(options map[string]string) {
	options[ManagedOption] = "1"
}

// ReadSection reads one section. A selector may be a named section or an
// anonymous selector with a unique match map. The returned bool is false when
// the section is absent.
func (c *Client) ReadSection(ctx context.Context, selector Section) (Section, bool, error) {
	if selector.Package == "" || selector.Type == "" || selector.Name == "" {
		return Section{}, false, errors.New("A section selector requires package, type and name.")
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, err := c.readRefreshState(ctx)
	if err != nil {
		return Section{}, false, err
	}
	if strings.HasPrefix(selector.Name, "@") && len(selector.Match) == 0 {
		var singleton *uciSection
		count := 0
		for index := range state[selector.Package] {
			actual := &state[selector.Package][index]
			if actual.Type == selector.Type {
				count++
				singleton = actual
			}
		}
		if count > 1 {
			return Section{}, false, errors.New("The anonymous selector matches multiple UCI sections. Use a unique identity.")
		}
		if count == 1 {
			result := Section{Package: selector.Package, Name: singleton.Name, Type: singleton.Type, Options: copyOptions(singleton.Options), Lists: copyLists(singleton.Lists)}
			return result, true, nil
		}
	}
	position := 0
	var result Section
	found := false
	for _, actual := range state[selector.Package] {
		matched := false
		if actual.Type == selector.Type {
			switch {
			case selector.Name != "" && !strings.HasPrefix(selector.Name, "@"):
				matched = actual.Name == selector.Name
			case len(selector.Match) > 0:
				matched = true
				for key, value := range selector.Match {
					if actual.Options[key] != value {
						matched = false
						break
					}
				}
			default:
				matched = actual.Name == selector.Name
			}
		}
		if matched {
			if found {
				return Section{}, false, errors.New("The section identity matches multiple UCI sections.")
			}
			result = Section{Package: selector.Package, Name: actual.Name, Type: actual.Type, Options: copyOptions(actual.Options), Lists: copyLists(actual.Lists)}
			if selector.Package == "firewall" && selector.Type == "rule" {
				value := position
				result.Position = &value
			}
			found = true
		}
		if selector.Package == "firewall" {
			position++
		}
	}
	return result, found, nil
}

func (c *Client) readRefreshState(ctx context.Context) (uciState, error) {
	for ctx.Err() == nil {
		value, err, shared := c.refresh.Do("uci", func() (any, error) {
			state, err := c.readSafetyState(ctx)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return state, err
		})
		if ctx.Err() != nil {
			break
		}
		if shared && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return value.(uciState), nil
	}
	return nil, ctx.Err()
}

// OwnerClaim is the ownership the caller observed before a write. The router
// re-checks it inside the transaction lock, so a section claimed by another
// state between the read and the write is refused instead of overwritten. An
// empty Token means the section had no owner token.
type OwnerClaim struct {
	Package string
	Section string
	Token   string
}

func validateOwnerClaim(claim *OwnerClaim) error {
	if claim == nil {
		return nil
	}
	if !validPackageName(claim.Package) || !validPackageName(claim.Section) {
		return errors.New("The ownership claim must name a valid UCI package and section.")
	}
	return nil
}

// AnonymousSectionName is the section name the provider creates for a missing
// anonymous selector. The provider can therefore claim the section before the
// first write, so a concurrent create is refused instead of overwritten.
func AnonymousSectionName(packageName, sectionType string, match map[string]string) string {
	identity, _ := json.Marshal([]any{packageName, sectionType, match})
	sum := sha256.Sum256(identity)
	return fmt.Sprintf("tf_glinet_%x", sum[:8])
}

// ApplySections writes the given sections as one recovery-protected UCI
// transaction. The full managed configuration (all sections carrying
// ManagedOption, plus the change set) is validated before mutation.
func (c *Client) ApplySections(ctx context.Context, changes Configuration, secrets map[string]string) error {
	return c.ApplySectionsOwned(ctx, changes, secrets, nil)
}

// ApplySectionsOwned is ApplySections with an ownership claim that is verified
// again after the router-side lock is acquired.
func (c *Client) ApplySectionsOwned(ctx context.Context, changes Configuration, secrets map[string]string, claim *OwnerClaim) error {
	if err := validateOwnerClaim(claim); err != nil {
		return err
	}
	if err := ValidateConfiguration(changes); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, section := range changes {
		if section.Absent {
			continue
		}
		if section.Options == nil {
			return errors.New("Typed resource changes must include section options.")
		}
	}
	state, err := c.readSafetyState(ctx)
	if err != nil {
		return err
	}
	full := configurationFromManagedState(state)
	full = mergeManagedConfiguration(full, changes)
	return c.writeConfiguration(ctx, changes, secrets, full, claim, nil)
}

// ForgetSection releases Terraform ownership while preserving the section.
// This is used for firmware-owned anonymous sections, which cannot be deleted
// without breaking the stock configuration. The owner token is cleared on the
// router so a later Create does not see a stale owner.
func (c *Client) ForgetSection(ctx context.Context, selector Section, claim *OwnerClaim) error {
	if selector.Options == nil {
		selector.Options = map[string]string{}
	}
	selector.Options[ManagedOption] = "0"
	selector.Options[OwnerTokenOption] = ""
	selector.Absent = false
	return c.ApplySectionsOwned(ctx, Configuration{selector}, nil, claim)
}

// FirewallRules returns every firewall rule in UCI package order. Position is
// the absolute section index, including non-rule sections. The provider keeps
// this ordering detail behind the ordered firewall rules resource.
func (c *Client) FirewallRules(ctx context.Context) (Configuration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, err := c.readRefreshState(ctx)
	if err != nil {
		return nil, err
	}
	rules := Configuration{}
	position := 0
	for _, actual := range state["firewall"] {
		if actual.Type == "rule" {
			value := position
			rules = append(rules, Section{
				Package:  "firewall",
				Name:     actual.Name,
				Type:     actual.Type,
				Options:  copyOptions(actual.Options),
				Lists:    copyLists(actual.Lists),
				Position: &value,
			})
		}
		position++
	}
	return rules, nil
}

// ApplyFirewallRules writes the complete ordered rule set in one recovery-
// protected configuration transaction. Every rule currently on the router must
// appear in desired or removed; this prevents a stock rule from disappearing
// silently when the ordered list is incomplete.
func (c *Client) ApplyFirewallRules(ctx context.Context, desired, removed Configuration) error {
	desired = append(Configuration{}, desired...)
	for i := range desired {
		desired[i].Position = nil
	}
	if err := ValidateConfiguration(desired); err != nil {
		return err
	}
	for _, section := range desired {
		if section.Package != "firewall" || section.Type != "rule" || section.Absent {
			return errors.New("The ordered firewall policy accepts only present firewall rules.")
		}
	}
	for _, section := range removed {
		if section.Package != "firewall" || section.Type != "rule" || !section.Absent {
			return errors.New("Removed firewall rules must be absent rule sections.")
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	state, err := c.readSafetyState(ctx)
	if err != nil {
		return err
	}
	actualRules := Configuration{}
	position := 0
	for _, actual := range state["firewall"] {
		if actual.Type == "rule" {
			value := position
			actualRules = append(actualRules, Section{
				Package:  "firewall",
				Name:     actual.Name,
				Type:     actual.Type,
				Options:  copyOptions(actual.Options),
				Lists:    copyLists(actual.Lists),
				Position: &value,
			})
		}
		position++
	}
	desiredIdentities := map[string]bool{}
	desiredNames := map[string]bool{}
	for _, section := range desired {
		identity := firewallRuleIdentity(section)
		if desiredIdentities[identity] {
			return errors.New("The ordered firewall policy contains a duplicate rule.")
		}
		desiredIdentities[identity] = true
		if !strings.HasPrefix(section.Name, "@") {
			if desiredNames[section.Name] {
				return errors.New("The ordered firewall policy contains a duplicate section.")
			}
			desiredNames[section.Name] = true
		}
	}
	removedNames := map[string]bool{}
	for _, section := range removed {
		if removedNames[section.Name] {
			return errors.New("A firewall rule cannot be removed twice.")
		}
		if desiredNames[section.Name] {
			return errors.New("A firewall rule cannot be updated and removed in the same change.")
		}
		removedNames[section.Name] = true
	}
	for _, actual := range actualRules {
		if !desiredIdentities[firewallRuleIdentity(actual)] && !desiredNames[actual.Name] && !removedNames[actual.Name] {
			return fmt.Errorf("The ordered firewall policy is missing rule %s. List it as an existing rule or remove it explicitly.", actual.Name)
		}
	}
	actualNames := map[string]bool{}
	for _, actual := range actualRules {
		actualNames[actual.Name] = true
	}
	for _, section := range removed {
		if !actualNames[section.Name] {
			return fmt.Errorf("Cannot remove missing firewall rule %s.", section.Name)
		}
	}
	lastPosition := position
	for i := range desired {
		var value int
		if i < len(actualRules) {
			value = *actualRules[i].Position
		} else {
			value = lastPosition + i - len(actualRules)
		}
		desired[i].Position = &value
	}
	changes := append(Configuration{}, desired...)
	changes = append(changes, removed...)
	if err := ValidateConfiguration(changes); err != nil {
		return err
	}
	full := configurationFromManagedState(state)
	full = mergeManagedConfiguration(full, changes)
	return c.writeConfiguration(ctx, changes, nil, full, nil, nil)
}

func firewallRuleIdentity(section Section) string {
	name := section.Options["name"]
	if name == "" {
		name = section.Name
	}
	return name + "\x00" + section.Options["src"] + "\x00" + section.Options["dest"]
}

func configurationFromManagedState(state uciState) Configuration {
	config := Configuration{}
	for packageName, sections := range state {
		for _, section := range sections {
			if !IsManaged(section.Options) {
				continue
			}
			config = append(config, Section{
				Package: packageName,
				Name:    section.Name,
				Type:    section.Type,
				Options: copyOptions(section.Options),
				Lists:   copyLists(section.Lists),
			})
		}
	}
	return config
}

func mergeManagedConfiguration(full, changes Configuration) Configuration {
	result := Configuration{}
	for _, section := range full {
		matched := false
		for _, change := range changes {
			if sectionMatches(change, section) {
				matched = true
				break
			}
		}
		if !matched {
			result = append(result, section)
		}
	}
	for _, change := range changes {
		if change.Absent {
			continue
		}
		result = append(result, change)
	}
	return result
}

func sectionMatches(selector, actual Section) bool {
	if selector.Package != actual.Package || selector.Type != actual.Type {
		return false
	}
	if selector.Name != "" && !strings.HasPrefix(selector.Name, "@") {
		return selector.Name == actual.Name
	}
	if len(selector.Match) > 0 {
		for key, value := range selector.Match {
			if actual.Options[key] != value {
				return false
			}
		}
		return true
	}
	return selector.Name == actual.Name
}

func copyOptions(options map[string]string) map[string]string {
	if options == nil {
		return nil
	}
	result := make(map[string]string, len(options))
	for key, value := range options {
		result[key] = value
	}
	return result
}

func copyLists(lists map[string][]string) map[string][]string {
	if lists == nil {
		return nil
	}
	result := make(map[string][]string, len(lists))
	for key, values := range lists {
		result[key] = append([]string{}, values...)
	}
	return result
}
