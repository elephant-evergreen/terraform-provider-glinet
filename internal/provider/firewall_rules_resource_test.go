package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFirewallRuleSectionRoundTrip(t *testing.T) {
	input := firewallRuleInput{
		Name:      "example-block-management",
		Src:       "wan",
		Dest:      "lan",
		SrcIPs:    []string{"192.0.2.10"},
		Protocols: []string{"tcp"},
		SrcPorts:  []string{"1024-2048"},
		DestPorts: []string{"22", "443"},
		Family:    "ipv4",
		Target:    "DROP",
		Enabled:   true,
	}
	section, err := firewallRuleSection(input)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "firewall" || section.Type != "rule" || section.Options["name"] != input.Name {
		t.Fatalf("unexpected section: %+v", section)
	}
	if section.Options["src_ip"] != "192.0.2.10" || section.Options["dest_port"] != "22 443" {
		t.Fatalf("rule options were not mapped: %+v", section.Options)
	}
	loaded := firewallRuleInputFromSection(section)
	if loaded.Name != input.Name || loaded.Target != input.Target || loaded.Family != input.Family {
		t.Fatalf("rule did not round trip: %+v", loaded)
	}
	if len(loaded.DestPorts) != 2 || len(loaded.DestIPs) != 0 {
		t.Fatalf("rule ports did not round trip: %+v", loaded)
	}
}

func TestParseFirewallRuleInputsOrder(t *testing.T) {
	first, diags := firewallRuleObject(firewallRuleInput{Name: "first", Existing: true, Family: "ipv4", Target: "ACCEPT", Enabled: true})
	if diags.HasError() {
		t.Fatal(diags)
	}
	second, diags := firewallRuleObject(firewallRuleInput{Name: "second", Family: "ipv4", Target: "REJECT", Enabled: true})
	if diags.HasError() {
		t.Fatal(diags)
	}
	list := types.ListValueMust(firewallRuleObjectType(), []attr.Value{first, second})
	inputs, diags := parseFirewallRuleInputs(context.Background(), list)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if len(inputs) != 2 || inputs[0].Name != "first" || !inputs[0].Existing || inputs[1].Name != "second" || inputs[1].Existing {
		t.Fatalf("unexpected inputs: %+v", inputs)
	}
}
