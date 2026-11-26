package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDDNSSpecRoundTrip(t *testing.T) {
	spec := NewDDNSResource().(*typedResource).spec
	values := map[string]attr.Value{
		"enabled_ssh": types.BoolValue(false), "http_port": types.Int64Value(80), "https_port": types.Int64Value(443),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "gl_ddns" || section.Type != "service" || section.Options["http_port"] != "80" {
		t.Fatalf("unexpected ddns section: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["https_port"].(types.Int64).ValueInt64() != 443 {
		t.Fatalf("unexpected ddns round trip: %+v", loaded)
	}
}

func TestDDNSPortValidation(t *testing.T) {
	spec := NewDDNSResource().(*typedResource).spec
	values := map[string]attr.Value{
		"enabled_ssh": types.BoolValue(false), "http_port": types.Int64Value(0), "https_port": types.Int64Value(443),
	}
	if err := spec.Validate(values); err == nil {
		t.Fatal("invalid DDNS port was accepted")
	}
}

func TestSambaSpecRoundTrip(t *testing.T) {
	spec := NewSambaResource().(*typedResource).spec
	values := map[string]attr.Value{
		"workgroup": types.StringValue("WORKGROUP"), "charset": types.StringValue("UTF-8"),
		"description": types.StringValue("Samba on OpenWRT"), "name": types.StringValue(""), "interfaces": types.StringValue(""),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "samba4" || section.Type != "samba" || section.Name != "@samba[0]" {
		t.Fatalf("unexpected samba section: %+v", section)
	}
}
