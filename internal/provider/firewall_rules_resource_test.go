package provider

import (
	"context"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

func TestFirewallRulesProtocol(t *testing.T) {
	remote := &typedFixtureRunner{}
	client := router.NewClient(remote, fixtureProtection{})
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := `resource "glinet_firewall_rules" "test" {
  rules = [{
    name       = "example-block-management"
    src        = "wan"
    protocols  = ["tcp"]
    dest_ports = ["22", "443"]
    target     = "DROP"
  }]
}`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			remote.mu.Lock()
			defer remote.mu.Unlock()
			if len(remote.actual) != 1 {
				t.Fatalf("destroy removed the managed rule instead of releasing it: %+v", remote.actual)
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config: config,
			Check: func(state *terraform.State) error {
				if len(remote.actual) != 1 || remote.actual[0].Options["name"] != "example-block-management" {
					t.Fatalf("unexpected applied rules: %+v", remote.actual)
				}
				return nil
			},
		}},
	})
}

func TestFirewallRulesExistingReferenceProtocol(t *testing.T) {
	remote := &typedFixtureRunner{actual: router.Configuration{{
		Package: "firewall", Name: "stock_rule", Type: "rule",
		Options: map[string]string{"name": "Stock-Rule", "target": "ACCEPT"},
	}}}
	client := router.NewClient(remote, fixtureProtection{})
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := `resource "glinet_firewall_rules" "test" {
  rules = [
    { name = "Stock-Rule", existing = true },
    {
      name       = "example-block-management"
      src        = "wan"
      protocols  = ["tcp"]
      dest_ports = ["22"]
      target     = "DROP"
    },
  ]
}`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config: config,
			Check: func(state *terraform.State) error {
				remote.mu.Lock()
				defer remote.mu.Unlock()
				if len(remote.actual) != 2 || remote.actual[0].Options["name"] != "Stock-Rule" {
					t.Fatalf("existing stock rule was not preserved first: %+v", remote.actual)
				}
				return nil
			},
		}},
	})
}

func TestFirewallRulesDuplicateStockNamesProtocol(t *testing.T) {
	remote := &typedFixtureRunner{actual: router.Configuration{
		{Package: "firewall", Name: "stock_guest", Type: "rule", Options: map[string]string{"name": "Allow-DHCP", "src": "guest", "target": "ACCEPT"}},
		{Package: "firewall", Name: "stock_iot", Type: "rule", Options: map[string]string{"name": "Allow-DHCP", "src": "iot", "target": "ACCEPT"}},
	}}
	client := router.NewClient(remote, fixtureProtection{})
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := `resource "glinet_firewall_rules" "test" {
  rules = [
    { name = "Allow-DHCP", existing = true, src = "guest" },
    { name = "Allow-DHCP", existing = true, src = "iot" },
    {
      name       = "example-block-management"
      src        = "wan"
      protocols  = ["tcp"]
      dest_ports = ["22"]
      target     = "DROP"
    },
  ]
}`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config: config,
			Check: func(state *terraform.State) error {
				remote.mu.Lock()
				defer remote.mu.Unlock()
				if len(remote.actual) != 3 || remote.actual[0].Options["src"] != "guest" || remote.actual[1].Options["src"] != "iot" {
					t.Fatalf("duplicate stock rules were not preserved by source: %+v", remote.actual)
				}
				return nil
			},
		}},
	})
}
