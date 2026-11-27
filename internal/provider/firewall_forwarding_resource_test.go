package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFirewallForwardingUpdatesInPlace(t *testing.T) {
	resource := NewFirewallForwardingResource().(*typedResource)
	plan := map[string]attr.Value{"src": stringValue("iot"), "dest": stringValue("wan"), "enabled": boolValue(true)}
	state := map[string]attr.Value{"src": stringValue("guest"), "dest": stringValue("wan"), "enabled": boolValue(true), "uci_section": stringValue("cfg0a1b2c")}
	section, err := resource.spec.UpdateSelector(plan, state)
	if err != nil {
		t.Fatal(err)
	}
	if section.Name != "cfg0a1b2c" || len(section.Match) != 0 {
		t.Fatalf("zone change did not target the existing section: %+v", section)
	}
	if section.Options["src"] != "iot" || section.Options["dest"] != "wan" {
		t.Fatalf("zone change did not update the options: %+v", section)
	}
	section, err = resource.spec.UpdateSelector(plan, map[string]attr.Value{"uci_section": types.StringNull()})
	if err != nil {
		t.Fatal(err)
	}
	if section.Name != "@forwarding[0]" || section.Match["src"] != "iot" {
		t.Fatalf("missing section name did not fall back to the identity match: %+v", section)
	}
}
