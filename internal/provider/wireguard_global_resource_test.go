package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestWireGuardGlobalSpecRoundTrip(t *testing.T) {
	spec := NewWireGuardGlobalResource().(*typedResource).spec
	values := map[string]attr.Value{"global_proxy": types.BoolValue(true)}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "wireguard" || section.Options["global_proxy"] != "1" {
		t.Fatalf("unexpected WireGuard global: %+v", section)
	}
}
