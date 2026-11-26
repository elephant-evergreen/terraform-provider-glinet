package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestTorSpecRoundTrip(t *testing.T) {
	spec := NewTorResource().(*typedResource).spec
	values := map[string]attr.Value{"enabled": types.BoolValue(true), "manual": types.BoolValue(false), "countries": types.StringValue("jp"), "ipv6_intercept": types.BoolValue(true)}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "tor" || section.Options["enable"] != "1" {
		t.Fatalf("unexpected Tor: %+v", section)
	}
}
