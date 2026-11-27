package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestNetworkModeSpecRoundTrip(t *testing.T) {
	spec := NewNetworkModeResource().(*typedResource).spec
	values := map[string]attr.Value{
		"mode": types.StringValue("router"), "autotimezone": types.BoolValue(true),
		"track_ip": stringSet([]string{"1.1.1.1"}), "track_ipv6": stringSet([]string{"2001:4860:4860::8888"}),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "glconfig" || section.Options["mode"] != "router" || len(section.Lists["track_ip"]) != 1 {
		t.Fatalf("unexpected network mode: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["mode"].(types.String).ValueString() != "router" {
		t.Fatalf("unexpected round trip: %+v", loaded)
	}
}

func TestNetworkModeValidation(t *testing.T) {
	spec := NewNetworkModeResource().(*typedResource).spec
	for _, mode := range []string{"", "ap", "repeater"} {
		if err := spec.Validate(map[string]attr.Value{
			"mode": types.StringValue(mode), "autotimezone": types.BoolValue(true),
			"track_ip": stringSet(nil), "track_ipv6": stringSet(nil),
		}); err == nil {
			t.Fatalf("mode %q was accepted", mode)
		}
	}
	if err := spec.Validate(map[string]attr.Value{
		"mode": types.StringValue("router"), "autotimezone": types.BoolValue(true),
		"track_ip": stringSet(nil), "track_ipv6": stringSet(nil),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDropInGatewaySpecRoundTrip(t *testing.T) {
	spec := NewDropInGatewayResource().(*typedResource).spec
	section, err := spec.Selector(map[string]attr.Value{
		"enabled": types.BoolValue(true), "force_dns": types.BoolValue(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "edgerouter" || section.Options["enabled"] != "1" {
		t.Fatalf("unexpected drop-in gateway: %+v", section)
	}
}

func TestDropInDHCPSpecRoundTrip(t *testing.T) {
	spec := NewDropInDHCPResource().(*typedResource).spec
	section, err := spec.Selector(map[string]attr.Value{
		"lease_time": types.Int64Value(43200), "ignore": types.BoolValue(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "edgerouter" || section.Type != "dhcp" || section.Options["leasetime"] != "43200" {
		t.Fatalf("unexpected drop-in DHCP: %+v", section)
	}
}
