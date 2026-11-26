package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWireGuardServerSpecRoundTrip(t *testing.T) {
	spec := NewWireGuardServerResource().(*typedResource).spec
	values := map[string]attr.Value{
		"name": types.StringValue("main_server"), "address_v4": types.StringValue("10.1.0.1/24"),
		"address_v6": types.StringValue("fd00::1/64"), "port": types.Int64Value(51820),
		"access": types.StringValue("DROP"), "client_to_client": types.BoolValue(false),
		"masq": types.BoolValue(true),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "wireguard_server" || section.Type != "servers" || section.Options["port"] != "51820" {
		t.Fatalf("unexpected WireGuard server: %+v", section)
	}
	loaded := spec.Load(section)
	if !loaded["masq"].(types.Bool).ValueBool() {
		t.Fatalf("unexpected round trip: %+v", loaded)
	}
}

func TestWireGuardServerValidation(t *testing.T) {
	spec := NewWireGuardServerResource().(*typedResource).spec
	values := map[string]attr.Value{
		"name": types.StringValue("main_server"), "address_v4": types.StringValue(""),
		"address_v6": types.StringValue(""), "port": types.Int64Value(0),
		"access": types.StringValue("ALLOW"), "client_to_client": types.BoolValue(false),
		"masq": types.BoolValue(true),
	}
	if err := spec.Validate(values); err == nil {
		t.Fatal("invalid WireGuard server was accepted")
	}
}

func TestS2SSpecRoundTrip(t *testing.T) {
	spec := NewS2SResource().(*typedResource).spec
	section, err := spec.Selector(map[string]attr.Value{
		"enabled": types.BoolValue(true), "firewall": types.BoolValue(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "gl_s2s" || section.Options["enable"] != "1" {
		t.Fatalf("unexpected s2s: %+v", section)
	}
}
