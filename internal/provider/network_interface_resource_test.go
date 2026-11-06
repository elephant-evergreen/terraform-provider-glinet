package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestNetworkInterfaceSpecRoundTrip(t *testing.T) {
	resource := NewNetworkInterfaceResource()
	_ = resource
	spec := typedNetworkInterfaceSpec()
	values := map[string]attr.Value{
		"name":         types.StringValue("vlan11"),
		"device":       types.StringValue("br-vlan11"),
		"protocol":     types.StringValue("static"),
		"address":      types.StringValue("192.168.11.1/24"),
		"gateway":      types.StringValue(""),
		"dns":          types.SetValueMust(types.StringType, []attr.Value{types.StringValue("9.9.9.9")}),
		"vlan_id":      types.Int64Value(11),
		"display_name": types.StringValue("NAS"),
		"force_link":   types.BoolValue(true),
		"delegate":     types.BoolValue(false),
		"enabled":      types.BoolValue(true),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Options["ipaddr"] != "192.168.11.1" || section.Options["netmask"] != "255.255.255.0" || section.Options["vlan_id"] != "11" || section.Options["disabled"] != "0" {
		t.Fatalf("unexpected section: %+v", section)
	}
	if len(section.Lists["dns"]) != 1 || section.Lists["dns"][0] != "9.9.9.9" {
		t.Fatalf("unexpected DNS list: %+v", section.Lists)
	}
	loaded := spec.Load(section)
	if loaded["address"].(types.String).ValueString() != "192.168.11.1/24" || loaded["vlan_id"].(types.Int64).ValueInt64() != 11 || !loaded["force_link"].(types.Bool).ValueBool() {
		t.Fatalf("unexpected round trip: %+v", loaded)
	}
}

func TestNetworkInterfaceValidation(t *testing.T) {
	spec := typedNetworkInterfaceSpec()
	base := map[string]attr.Value{
		"name":         types.StringValue("guest"),
		"device":       types.StringValue("br-guest"),
		"protocol":     types.StringValue("static"),
		"address":      types.StringValue("192.168.9.1/24"),
		"gateway":      types.StringValue(""),
		"dns":          types.SetValueMust(types.StringType, nil),
		"vlan_id":      types.Int64Value(0),
		"display_name": types.StringValue(""),
		"force_link":   types.BoolValue(false),
		"delegate":     types.BoolValue(false),
		"enabled":      types.BoolValue(true),
	}
	for name, mutate := range map[string]func(map[string]attr.Value){
		"dhcp address":    func(values map[string]attr.Value) { values["protocol"] = types.StringValue("dhcp") },
		"missing address": func(values map[string]attr.Value) { values["address"] = types.StringValue("") },
		"bad vlan":        func(values map[string]attr.Value) { values["vlan_id"] = types.Int64Value(5000) },
	} {
		t.Run(name, func(t *testing.T) {
			values := map[string]attr.Value{}
			for key, value := range base {
				values[key] = value
			}
			mutate(values)
			if err := spec.Validate(values); err == nil {
				t.Fatal("invalid interface was accepted")
			}
		})
	}
}
