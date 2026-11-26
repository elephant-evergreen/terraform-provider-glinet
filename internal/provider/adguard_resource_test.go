package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestAdGuardSpecRoundTrip(t *testing.T) {
	spec := NewAdGuardResource().(*typedResource).spec
	values := map[string]attr.Value{"enabled": types.BoolValue(true), "override_vpn": types.BoolValue(false)}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "adguardhome" || section.Options["enabled"] != "1" {
		t.Fatalf("unexpected AdGuard: %+v", section)
	}
}
