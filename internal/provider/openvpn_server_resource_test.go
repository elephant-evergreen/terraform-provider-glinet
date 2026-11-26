package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestOpenVPNServerSpecRoundTrip(t *testing.T) {
	spec := NewOpenVPNServerResource().(*typedResource).spec
	values := map[string]attr.Value{"name": types.StringValue("vpn"), "enabled": types.BoolValue(true), "port": types.Int64Value(1194), "proto": types.StringValue("udp"), "dev": types.StringValue("tun"), "server": types.StringValue("10.8.0.0 255.255.255.0"), "ca": types.StringValue(""), "cert": types.StringValue(""), "key": types.StringValue(""), "dh": types.StringValue("")}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "openvpn" || section.Options["port"] != "1194" {
		t.Fatalf("unexpected OpenVPN server: %+v", section)
	}
}
