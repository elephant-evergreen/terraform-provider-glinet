package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRoutePolicyRuleSpecRoundTrip(t *testing.T) {
	spec := NewRoutePolicyRuleResource().(*typedResource).spec
	values := map[string]attr.Value{
		"name": types.StringValue("gl_process"), "kind": types.StringValue("rule_process"),
		"enabled": types.BoolValue(true), "via": types.StringValue("wgclient1"),
		"from_type": types.StringValue("process_gid"), "from": stringSet([]string{"10000"}),
		"tunnel_id": types.Int64Value(201), "mark": types.StringValue(""),
		"label": types.StringValue("process policy"),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "route_policy" || section.Type != "rule_process" || section.Options["tunnel_id"] != "201" {
		t.Fatalf("unexpected policy rule: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["from_type"].(types.String).ValueString() != "process_gid" {
		t.Fatalf("unexpected round trip: %+v", loaded)
	}
}

func TestRoutePolicyRuleDefaultKind(t *testing.T) {
	spec := NewRoutePolicyRuleResource().(*typedResource).spec
	section, err := spec.Selector(map[string]attr.Value{
		"name": types.StringValue("default"), "kind": types.StringValue("default"),
		"enabled": types.BoolValue(true), "via": types.StringValue("novpn"),
		"from_type": types.StringValue("device"), "from": stringSet(nil),
		"tunnel_id": types.Int64Value(100), "mark": types.StringValue("0x8000"),
		"label": types.StringValue("last sort default policy"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if section.Type != "default" || section.Options["type"] != "default" {
		t.Fatalf("unexpected default rule: %+v", section)
	}
}

func TestRoutePolicyRuleValidation(t *testing.T) {
	spec := NewRoutePolicyRuleResource().(*typedResource).spec
	if _, err := spec.Selector(map[string]attr.Value{
		"name": types.StringValue("x"), "kind": types.StringValue("other"),
		"enabled": types.BoolValue(true), "via": types.StringValue(""),
		"from_type": types.StringValue(""), "from": stringSet(nil),
		"tunnel_id": types.Int64Value(0), "mark": types.StringValue(""),
		"label": types.StringValue(""),
	}); err == nil {
		t.Fatal("invalid rule kind was accepted")
	}
}
