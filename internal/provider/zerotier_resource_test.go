package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestZeroTierSpecRoundTrip(t *testing.T) {
	spec := NewZeroTierResource().(*typedResource).spec
	values := map[string]attr.Value{"enabled": types.BoolValue(true), "nat": types.BoolValue(false), "wan_nat": types.BoolValue(false)}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "zerotier" || section.Options["enabled"] != "1" {
		t.Fatalf("unexpected ZeroTier: %+v", section)
	}
}
