package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWireGuardInterfaceSpecMapping(t *testing.T) {
	spec := typedWireGuardInterfaceSpec()
	key := strings.Repeat("a", 44)
	values := map[string]attr.Value{
		"name":                types.StringValue("wg0"),
		"addresses":           stringSet([]string{"10.8.0.1/24"}),
		"listen_port":         types.Int64Value(51820),
		"mtu":                 types.Int64Value(1420),
		"fwmark":              types.StringValue(""),
		"no_host_route":       types.BoolValue(false),
		"enabled":             types.BoolValue(true),
		"private_key_wo":      types.StringValue(key),
		"private_key_version": types.Int64Value(1),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Type != "interface" || section.Options["proto"] != "wireguard" || section.Lists["addresses"][0] != "10.8.0.1/24" {
		t.Fatalf("unexpected interface section: %+v", section)
	}
	secrets := spec.Secrets(values)
	if secrets["network.wg0.private_key"] != key || !router.ValidSecretOptions(router.Configuration{section}, secrets) {
		t.Fatalf("unexpected interface secret: %+v", secrets)
	}
	loaded := spec.Load(router.Section{Package: "network", Type: "interface", Name: "wg0", Options: section.Options, Lists: section.Lists})
	if loaded["name"].(types.String).ValueString() != "wg0" || loaded["private_key_wo"].(types.String).IsNull() == false {
		t.Fatalf("unexpected interface round trip: %+v", loaded)
	}
}

func TestWireGuardPeerSpecMapping(t *testing.T) {
	spec := typedWireGuardPeerSpec()
	key := strings.Repeat("b", 44)
	values := map[string]attr.Value{
		"section":               types.StringValue("peer1"),
		"interface":             types.StringValue("wg0"),
		"public_key":            types.StringValue(strings.Repeat("c", 44)),
		"allowed_ips":           stringSet([]string{"10.8.0.2/32"}),
		"endpoint_host":         types.StringValue("vpn.example"),
		"endpoint_port":         types.Int64Value(51820),
		"persistent_keepalive":  types.Int64Value(25),
		"route_allowed_ips":     types.BoolValue(true),
		"description":           types.StringValue("example"),
		"preshared_key_wo":      types.StringValue(key),
		"preshared_key_version": types.Int64Value(1),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Type != "wireguard_wg0" || section.Options["public_key"] == "" || section.Lists["allowed_ips"][0] != "10.8.0.2/32" {
		t.Fatalf("unexpected peer section: %+v", section)
	}
	if _, err := router.DecodeConfiguration(configurationJSONForTest(section)); err != nil {
		t.Fatalf("dynamic WireGuard peer type rejected: %v", err)
	}
	secrets := spec.Secrets(values)
	if secrets["network.peer1.preshared_key"] != key || !router.ValidSecretOptions(router.Configuration{section}, secrets) {
		t.Fatalf("unexpected peer secret: %+v", secrets)
	}
	loaded := spec.Load(section)
	if loaded["interface"].(types.String).ValueString() != "wg0" || loaded["section"].(types.String).ValueString() != "peer1" {
		t.Fatalf("unexpected peer round trip: %+v", loaded)
	}
}

func configurationJSONForTest(sections ...router.Section) string {
	encoded, _ := json.Marshal(sections)
	return string(encoded)
}
