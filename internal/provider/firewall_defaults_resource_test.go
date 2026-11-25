package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFirewallDefaultsSpecRoundTrip(t *testing.T) {
	spec := NewFirewallDefaultsResource().(*typedResource).spec
	values := map[string]attr.Value{
		"input": types.StringValue("DROP"), "output": types.StringValue("ACCEPT"), "forward": types.StringValue("DROP"),
		"syn_flood": types.BoolValue(false), "synflood_protect": types.BoolValue(true),
		"synflood_rate": types.StringValue("25/s"), "synflood_burst": types.StringValue("50"),
		"drop_invalid": types.BoolValue(true), "tcp_syncookies": types.BoolValue(true),
		"flow_offloading": types.BoolValue(true), "flow_offloading_hw": types.BoolValue(false),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "firewall" || section.Type != "defaults" || section.Options["input"] != "DROP" || section.Options["drop_invalid"] != "1" {
		t.Fatalf("unexpected defaults section: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["input"].(types.String).ValueString() != "DROP" || !loaded["drop_invalid"].(types.Bool).ValueBool() || !loaded["flow_offloading"].(types.Bool).ValueBool() {
		t.Fatalf("unexpected defaults round trip: %+v", loaded)
	}
}

func TestFirewallDefaultsValidation(t *testing.T) {
	spec := NewFirewallDefaultsResource().(*typedResource).spec
	values := map[string]attr.Value{
		"input": types.StringValue("ALLOW"), "output": types.StringValue("ACCEPT"), "forward": types.StringValue("REJECT"),
		"syn_flood": types.BoolValue(true), "synflood_protect": types.BoolValue(true),
		"synflood_rate": types.StringValue("25/s"), "synflood_burst": types.StringValue("50"),
		"drop_invalid": types.BoolValue(false), "tcp_syncookies": types.BoolValue(true),
		"flow_offloading": types.BoolValue(false), "flow_offloading_hw": types.BoolValue(false),
	}
	if err := spec.Validate(values); err == nil {
		t.Fatal("invalid default policy was accepted")
	}
}
