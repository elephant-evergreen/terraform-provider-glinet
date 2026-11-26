package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestQoSSpecRoundTrip(t *testing.T) {
	spec := NewQoSResource().(*typedResource).spec
	values := map[string]attr.Value{"enabled": types.BoolValue(true), "model": types.StringValue("cake"), "upload": types.Int64Value(1000), "download": types.Int64Value(1000), "sum": types.BoolValue(false)}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "qos" || section.Options["enable"] != "1" {
		t.Fatalf("unexpected QoS: %+v", section)
	}
}
