package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestNetworkRuleSpecRoundTrip(t *testing.T) {
	spec := NewNetworkRuleResource().(*typedResource).spec
	values := map[string]attr.Value{
		"name": types.StringValue("work_table"), "lookup": types.StringValue("100"), "priority": types.Int64Value(1000),
		"src": types.StringValue("10.0.0.0/8"), "dest": types.StringValue(""), "in": types.StringValue("br-lan"),
		"out": types.StringValue(""), "family": types.StringValue("ipv4"), "invert": types.BoolValue(false),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "network" || section.Type != "rule" || section.Options["lookup"] != "100" || section.Options["priority"] != "1000" {
		t.Fatalf("unexpected rule: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["lookup"].(types.String).ValueString() != "100" || loaded["priority"].(types.Int64).ValueInt64() != 1000 {
		t.Fatalf("unexpected round trip: %+v", loaded)
	}
}

func TestNetworkRuleValidation(t *testing.T) {
	spec := NewNetworkRuleResource().(*typedResource).spec
	values := map[string]attr.Value{
		"name": types.StringValue("bad"), "lookup": types.StringValue(""), "priority": types.Int64Value(0),
		"src": types.StringValue(""), "dest": types.StringValue(""), "in": types.StringValue(""), "out": types.StringValue(""),
		"family": types.StringValue("ipv4"), "invert": types.BoolValue(false),
	}
	if err := spec.Validate(values); err == nil {
		t.Fatal("missing lookup was accepted")
	}
}
