package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestSQMSpecRoundTrip(t *testing.T) {
	spec := NewSQMResource().(*typedResource).spec
	values := map[string]attr.Value{"name": types.StringValue("wan"), "interface": types.StringValue("eth1"), "enabled": types.BoolValue(true), "qdisc": types.StringValue("cake"), "upload": types.Int64Value(100000), "download": types.Int64Value(500000), "script": types.StringValue("piece_of_cake.qos"), "linklayer": types.StringValue("none")}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "sqm" || section.Type != "queue" || section.Options["upload"] != "100000" {
		t.Fatalf("unexpected SQM: %+v", section)
	}
}
