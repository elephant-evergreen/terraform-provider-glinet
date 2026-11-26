package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSystemLEDSpecRoundTrip(t *testing.T) {
	spec := NewSystemLEDResource().(*typedResource).spec
	values := map[string]attr.Value{
		"name": types.StringValue("wan"), "sysfs": types.StringValue("green:wan"), "trigger": types.StringValue("netdev"),
		"dev": types.StringValue("eth1"), "mode": types.StringValue("link"), "default": types.StringValue("1"),
		"delayon": types.StringValue(""), "delayoff": types.StringValue(""),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "system" || section.Type != "led" || section.Options["trigger"] != "netdev" {
		t.Fatalf("unexpected LED: %+v", section)
	}
}
