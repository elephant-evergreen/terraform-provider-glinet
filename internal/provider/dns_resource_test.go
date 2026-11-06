package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDNSSelectorWritesStockProviderOption(t *testing.T) {
	spec := NewDNSResource().(*typedResource).spec
	section, err := spec.Selector(map[string]attr.Value{
		"dns_provider":     types.StringValue("quad9"),
		"protocol":         types.StringValue("doh"),
		"mode":             types.StringValue("secure"),
		"servers":          stringSet([]string{"quad9-doh-ip4-port443-filter-pri"}),
		"manual_enable":    types.BoolValue(true),
		"manual_type":      types.StringValue("secure"),
		"manual_local_dns": types.BoolValue(true),
		"override_vpn":     types.BoolValue(false),
		"force_dns":        types.BoolValue(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if section.Options["provider"] != "quad9" {
		t.Fatalf("provider option = %q, want quad9", section.Options["provider"])
	}
	if _, ok := section.Options["dns_provider"]; ok {
		t.Fatal("dns_provider is not a stock gl-dns-v2 option")
	}
}
