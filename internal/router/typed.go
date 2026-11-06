package router

import (
	"context"
	"errors"
	"strings"
)

// ManagedOption marks a UCI section as owned by a typed Terraform resource.
// It is written in the same atomic transaction as the section itself, so the
// safety checks can distinguish managed sections from firmware-managed ones
// without a separate registry.
const ManagedOption = "home_iac_owner"

// ReadSection reads one section. A selector may be a named section or an
// anonymous selector with a unique match map. The returned bool is false when
// the section is absent.
func (c *Client) ReadSection(ctx context.Context, selector Section) (Section, bool, error) {
	if selector.Package == "" || selector.Type == "" || selector.Name == "" {
		return Section{}, false, errors.New("A section selector requires package, type and name.")
	}
	state, err := c.readSafetyState(ctx)
	if err != nil {
		return Section{}, false, err
	}
	position := 0
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
			result := Section{Package: selector.Package, Name: actual.Name, Type: actual.Type, Options: copyOptions(actual.Options), Lists: copyLists(actual.Lists)}
			if selector.Package == "firewall" && selector.Type == "rule" {
				value := position
				result.Position = &value
			}
			return result, true, nil
		}
		if selector.Package == "firewall" {
			position++
		}
	}
	return Section{}, false, nil
}

// ApplySections writes the given sections as one recovery-protected UCI
// transaction. The full managed configuration (all sections carrying
// ManagedOption, plus the change set) is validated before mutation.
func (c *Client) ApplySections(ctx context.Context, changes Configuration, keys, secrets map[string]string) error {
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
	return c.writeConfiguration(ctx, changes, keys, secrets, full)
}

// ForgetSection releases Terraform ownership while preserving the section.
// This is used for firmware-owned anonymous sections, which cannot be deleted
// without breaking the stock configuration.
func (c *Client) ForgetSection(ctx context.Context, selector Section) error {
	if selector.Options == nil {
		selector.Options = map[string]string{}
	}
	selector.Options[ManagedOption] = "0"
	selector.Absent = false
	return c.ApplySections(ctx, Configuration{selector}, nil, nil)
}

func configurationFromManagedState(state uciState) Configuration {
	config := Configuration{}
	for packageName, sections := range state {
		for _, section := range sections {
			if section.Options[ManagedOption] != "1" {
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

// MarkConfiguration adds the provider ownership marker to every non-absent
// section. The legacy coordinated resource uses this to adopt all of its
// sections atomically before typed resources take over.
func MarkConfiguration(config Configuration) Configuration {
	result := make(Configuration, 0, len(config))
	for _, section := range config {
		next := section
		if !next.Absent {
			next.Options = copyOptions(next.Options)
			if next.Options == nil {
				next.Options = map[string]string{}
			}
			next.Options[ManagedOption] = "1"
		}
		result = append(result, next)
	}
	return result
}

// StripManagedMarker removes the internal ownership marker before comparing
// legacy state with the router or writing Terraform state.
func StripManagedMarker(config Configuration) Configuration {
	result := make(Configuration, 0, len(config))
	for _, section := range config {
		next := section
		next.Options = copyOptions(section.Options)
		if next.Options != nil {
			delete(next.Options, ManagedOption)
			if len(next.Options) == 0 {
				next.Options = nil
			}
		}
		result = append(result, next)
	}
	return result
}
