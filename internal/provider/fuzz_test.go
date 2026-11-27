package provider

import (
	"strings"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
)

func FuzzFirewallRuleRoundTrip(f *testing.F) {
	f.Add("allow-web", "lan", "wan", true, "ipv4", "ACCEPT")
	f.Add("", "guest", "nas", false, "", "")
	f.Add("名前", "*", "*", true, "any", "DROP")
	f.Fuzz(func(t *testing.T, name, src, dest string, enabled bool, family, target string) {
		if name == "" {
			return
		}
		input := firewallRuleInput{Name: name, Src: src, Dest: dest, Enabled: enabled, Family: family, Target: target}
		section, err := firewallRuleSection(input)
		if err != nil {
			return
		}
		decoded := firewallRuleInputFromSection(section)
		if decoded.Name != name || decoded.Src != src || decoded.Dest != dest || decoded.Enabled != enabled {
			t.Fatalf("rule fields did not round-trip: %#v -> %#v", input, decoded)
		}
		if decoded.Family != section.Options["family"] || decoded.Target != section.Options["target"] {
			t.Fatalf("rule family or target did not round-trip: %#v", decoded)
		}
	})
}

func FuzzReconcileFirewallRules(f *testing.F) {
	f.Add("web|lan|wan|1\nmail|lan|wan|0", "web|lan|wan|0", "rule-a|web|lan|wan")
	f.Add("", "", "")
	f.Add("same|lan|wan|0\nsame|lan|nas|0", "same|lan|wan|0", "rule-a|same|lan|wan")
	f.Fuzz(func(t *testing.T, planSpec, stateSpec, actualSpec string) {
		plan := decodeRuleInputs(planSpec)
		state := decodeRuleInputs(stateSpec)
		actual := decodeActualRules(actualSpec)
		desired, removed, err := reconcileFirewallRules(plan, state, actual)
		if err != nil {
			return
		}
		if len(desired) != len(plan) {
			t.Fatalf("reconciled %d of %d planned rules", len(desired), len(plan))
		}
		actualNames := map[string]bool{}
		for _, section := range actual {
			actualNames[section.Name] = true
		}
		for _, section := range desired {
			if section.Absent {
				t.Fatal("desired rule is absent")
			}
		}
		for _, section := range removed {
			if !section.Absent || section.Package != "firewall" || section.Type != "rule" {
				t.Fatalf("removed section is not an absent rule: %#v", section)
			}
			if !actualNames[section.Name] {
				t.Fatalf("removed rule %q is not on the router", section.Name)
			}
		}
	})
}

func decodeRuleInputs(spec string) []firewallRuleInput {
	var inputs []firewallRuleInput
	for _, line := range strings.Split(strings.TrimSpace(spec), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		input := firewallRuleInput{}
		if len(parts) > 0 {
			input.Name = parts[0]
		}
		if len(parts) > 1 {
			input.Src = parts[1]
		}
		if len(parts) > 2 {
			input.Dest = parts[2]
		}
		if len(parts) > 3 {
			input.Existing = parts[3] == "1"
		}
		inputs = append(inputs, input)
	}
	return inputs
}

func decodeActualRules(spec string) router.Configuration {
	var rules router.Configuration
	for _, line := range strings.Split(strings.TrimSpace(spec), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		section := router.Section{Package: "firewall", Type: "rule", Options: map[string]string{"target": "REJECT", "family": "ipv4", "enabled": "1"}}
		if len(parts) > 0 {
			section.Name = parts[0]
			section.Options["name"] = parts[0]
		}
		if len(parts) > 1 {
			section.Options["src"] = parts[1]
		}
		if len(parts) > 2 {
			section.Options["dest"] = parts[2]
		}
		rules = append(rules, section)
	}
	return rules
}
