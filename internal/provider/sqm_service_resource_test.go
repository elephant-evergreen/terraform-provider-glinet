package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestSQMSvcSpecRoundTrip(t *testing.T) {
	spec := NewSQMServiceResource().(*typedResource).spec
	values := map[string]attr.Value{"enabled": types.BoolValue(true), "autorate": types.BoolValue(false), "upload": types.Int64Value(0), "download": types.Int64Value(0), "qdisc": types.StringValue("cake")}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "sqm" || section.Type != "service" || section.Options["enabled"] != "1" {
		t.Fatalf("unexpected SQM service: %+v", section)
	}
}
