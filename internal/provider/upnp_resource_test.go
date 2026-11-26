package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestUPnPSpecRoundTrip(t *testing.T) {
	spec := NewUPnPResource().(*typedResource).spec
	values := map[string]attr.Value{"enabled": types.BoolValue(true), "internal_iface": types.StringValue("br-lan"), "external_iface": types.StringValue("wan"), "secure_mode": types.BoolValue(true)}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "upnpd" || section.Options["enabled"] != "1" {
		t.Fatalf("unexpected UPnP: %+v", section)
	}
}
