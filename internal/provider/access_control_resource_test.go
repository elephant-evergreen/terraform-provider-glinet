package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestClientFilterSpecRoundTrip(t *testing.T) {
	spec := NewClientFilterResource().(*typedResource).spec
	section, err := spec.Selector(map[string]attr.Value{
		"enabled": types.BoolValue(true), "mode": types.StringValue("white"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "gl-black_white_list" || section.Options["mode"] != "white" {
		t.Fatalf("unexpected client filter: %+v", section)
	}
}

func TestClientFilterValidation(t *testing.T) {
	spec := NewClientFilterResource().(*typedResource).spec
	if err := spec.Validate(map[string]attr.Value{
		"enabled": types.BoolValue(true), "mode": types.StringValue("grey"),
	}); err == nil {
		t.Fatal("invalid filter mode was accepted")
	}
}

func TestWANAccessSpecRoundTrip(t *testing.T) {
	spec := NewWANAccessResource().(*typedResource).spec
	section, err := spec.Selector(map[string]attr.Value{"whitelist": types.BoolValue(true)})
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "wan-access" || section.Options["whitelist"] != "1" {
		t.Fatalf("unexpected WAN access: %+v", section)
	}
}

func TestParentalControlSpecRoundTrip(t *testing.T) {
	spec := NewParentalControlResource().(*typedResource).spec
	values := map[string]attr.Value{
		"enabled": types.BoolValue(true), "drop_anonymous": types.BoolValue(false),
		"auto_update": types.BoolValue(false), "log_level": types.Int64Value(2),
		"source_devices": stringSet([]string{"br-lan", "br-guest"}),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "parental_control_v2" || len(section.Lists["src_dev"]) != 2 {
		t.Fatalf("unexpected parental control: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["log_level"].(types.Int64).ValueInt64() != 2 {
		t.Fatalf("unexpected parental control round trip: %+v", loaded)
	}
}
