package provider

import (
	"fmt"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
)

func BenchmarkFirewallRuleSection(b *testing.B) {
	input := firewallRuleInput{Name: "allow-web", Src: "lan", Dest: "wan", Protocols: []string{"tcp"}, DestPorts: []string{"443"}, Family: "ipv4", Target: "ACCEPT", Enabled: true}
	for i := 0; i < b.N; i++ {
		_, _ = firewallRuleSection(input)
	}
}

func BenchmarkReconcileFirewallRules(b *testing.B) {
	var plan, state []firewallRuleInput
	var actual router.Configuration
	for i := 0; i < 32; i++ {
		name := fmt.Sprintf("rule%d", i)
		input := firewallRuleInput{Name: name, Src: "lan", Dest: "wan", Family: "ipv4", Target: "REJECT", Enabled: true}
		plan = append(plan, input)
		state = append(state, input)
		actual = append(actual, router.Section{Package: "firewall", Name: fmt.Sprintf("section%d", i), Type: "rule", Options: map[string]string{"name": name, "src": "lan", "dest": "wan", "target": "REJECT", "family": "ipv4", "enabled": "1"}})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = reconcileFirewallRules(plan, state, actual)
	}
}
