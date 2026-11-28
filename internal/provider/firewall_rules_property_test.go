package provider

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

// TestReconcileFirewallRulesPreservesPlan covers the ordered firewall policy,
// which is the resource that does not use the shared typedResource path.
func TestReconcileFirewallRulesPreservesPlan(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(0, 6).Draw(t, "rules")
		plan := make([]firewallRuleInput, 0, count)
		for index := 0; index < count; index++ {
			plan = append(plan, firewallRuleInput{
				Name:    fmt.Sprintf("rule-%d", index),
				Src:     "lan",
				Dest:    "wan",
				Enabled: true,
				Family:  "ipv4",
				Target:  "REJECT",
			})
		}
		desired, removed, err := reconcileFirewallRules(plan, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(removed) != 0 {
			t.Fatalf("a fresh policy removed %d rules", len(removed))
		}
		if len(desired) != len(plan) {
			t.Fatalf("a fresh policy kept %d of %d rules", len(desired), len(plan))
		}
		for index := range plan {
			if desired[index].Options["name"] != plan[index].Name {
				t.Fatalf("rule order changed at %d: %#v", index, desired[index])
			}
			if desired[index].Absent || desired[index].Package != "firewall" || desired[index].Type != "rule" {
				t.Fatalf("planned rule %d is not a present firewall rule: %#v", index, desired[index])
			}
		}
		converged, removedAgain, err := reconcileFirewallRules(plan, plan, desired)
		if err != nil {
			t.Fatalf("reapplying the same policy did not converge: %v", err)
		}
		if len(removedAgain) != 0 || len(converged) != len(plan) {
			t.Fatalf("reapplying the same policy removed %d rules and kept %d", len(removedAgain), len(converged))
		}
	})
}
