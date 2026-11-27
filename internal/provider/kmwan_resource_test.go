package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestKMWANSpecRoundTrip(t *testing.T) {
	spec := NewKMWANResource().(*typedResource).spec
	values := map[string]attr.Value{
		"enabled": types.BoolValue(true), "mode": types.StringValue("failover"),
		"level": types.StringValue("medium"), "sensitivity": types.Int64Value(3000),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "kmwan" || section.Type != "global" || section.Options["enable"] != "1" || section.Options["sensitivity"] != "3000" {
		t.Fatalf("unexpected kmwan section: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["mode"].(types.String).ValueString() != "failover" || loaded["sensitivity"].(types.Int64).ValueInt64() != 3000 {
		t.Fatalf("unexpected kmwan round trip: %+v", loaded)
	}
}

func TestKMWANMemberSpecRoundTrip(t *testing.T) {
	spec := NewKMWANMemberResource().(*typedResource).spec
	values := map[string]attr.Value{
		"name": types.StringValue("secondwan"), "interface": types.StringValue("secondwan"),
		"metric": types.Int64Value(15), "weight": types.Int64Value(1),
		"track_mode": types.StringValue("force"), "addr_type": types.StringValue("4"),
		"disabled": types.BoolValue(false), "check": types.BoolValue(true),
		"tracks": stringSet([]string{"ping,1.1.1.1"}),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "kmwan" || section.Type != "member" || section.Lists["tracks"][0] != "ping,1.1.1.1" {
		t.Fatalf("unexpected kmwan member: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["metric"].(types.Int64).ValueInt64() != 15 || !loaded["check"].(types.Bool).ValueBool() {
		t.Fatalf("unexpected member round trip: %+v", loaded)
	}
}

func TestKMWANMemberValidation(t *testing.T) {
	spec := NewKMWANMemberResource().(*typedResource).spec
	base := map[string]attr.Value{
		"name": types.StringValue("wan"), "interface": types.StringValue("wan"),
		"metric": types.Int64Value(10), "weight": types.Int64Value(1),
		"track_mode": types.StringValue("force"), "addr_type": types.StringValue("4"),
		"disabled": types.BoolValue(false), "check": types.BoolValue(true),
		"tracks": stringSet(nil),
	}
	for name, mutate := range map[string]func(map[string]attr.Value){
		"missing interface": func(values map[string]attr.Value) { values["interface"] = types.StringValue("") },
		"bad family":        func(values map[string]attr.Value) { values["addr_type"] = types.StringValue("5") },
		"zero weight":       func(values map[string]attr.Value) { values["weight"] = types.Int64Value(0) },
	} {
		t.Run(name, func(t *testing.T) {
			values := map[string]attr.Value{}
			for key, value := range base {
				values[key] = value
			}
			mutate(values)
			if err := spec.Validate(values); err == nil {
				t.Fatal("invalid kmwan member was accepted")
			}
		})
	}
}

func TestRoutePolicySpecRoundTrip(t *testing.T) {
	spec := NewRoutePolicyResource().(*typedResource).spec
	values := map[string]attr.Value{
		"enabled": types.BoolValue(true), "mode": types.StringValue("0"),
		"append_source_if": stringSet([]string{"iot"}), "instance_on": types.BoolValue(false),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "route_policy" || section.Type != "policy" || len(section.Lists["append_source_if"]) != 1 || section.Lists["append_source_if"][0] != "iot" {
		t.Fatalf("unexpected route policy: %+v", section)
	}
}

func TestKMWANModeValidation(t *testing.T) {
	spec := NewKMWANResource().(*typedResource).spec
	values := map[string]attr.Value{
		"enabled": types.BoolValue(true), "mode": types.StringValue("roundrobin"),
		"level": types.StringValue("medium"), "sensitivity": types.Int64Value(3000),
	}
	if err := spec.Validate(values); err == nil {
		t.Fatal("invalid multi-WAN mode was accepted")
	}
}
