package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestMiniDLNASpecRoundTrip(t *testing.T) {
	spec := NewMiniDLNAResource().(*typedResource).spec
	values := map[string]attr.Value{"enabled": types.BoolValue(true), "port": types.Int64Value(8200), "interface": types.StringValue("br-lan"), "friendly_name": types.StringValue("DLNA"), "media_dir": types.StringValue("/mnt")}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "minidlna" || section.Options["enabled"] != "1" {
		t.Fatalf("unexpected MiniDLNA: %+v", section)
	}
}
