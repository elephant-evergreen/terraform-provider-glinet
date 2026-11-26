package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestOpenVPNSpecRoundTrip(t *testing.T) {
	spec := NewOpenVPNResource().(*typedResource).spec
	values := map[string]attr.Value{"enabled": types.BoolValue(true), "config": types.StringValue("/etc/openvpn/my-vpn.conf")}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "openvpn" || section.Options["enabled"] != "1" {
		t.Fatalf("unexpected OpenVPN: %+v", section)
	}
}
