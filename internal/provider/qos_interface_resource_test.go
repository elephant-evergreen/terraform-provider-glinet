package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestQoSInterfaceSpecRoundTrip(t *testing.T) {
	spec := NewQoSInterfaceResource().(*typedResource).spec
	values := map[string]attr.Value{"name": types.StringValue("wan"), "kind": types.StringValue("wan"), "upload": types.Int64Value(1000), "download": types.Int64Value(1000)}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "qos" || section.Type != "wan" {
		t.Fatalf("unexpected QoS interface: %+v", section)
	}
}
