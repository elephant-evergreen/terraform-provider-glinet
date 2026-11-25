package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestNetworkRouteSpecRoundTrip(t *testing.T) {
	spec := NewNetworkRouteResource().(*typedResource).spec
	values := map[string]attr.Value{
		"name":      types.StringValue("nas_route"),
		"interface": types.StringValue("lan"),
		"target":    types.StringValue("10.20.0.0/16"),
		"gateway":   types.StringValue("192.168.1.1"),
		"metric":    types.Int64Value(10),
		"table":     types.StringValue(""),
		"source":    types.StringValue(""),
		"onlink":    types.BoolValue(true),
		"mtu":       types.Int64Value(1400),
		"type":      types.StringValue("unicast"),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "network" || section.Type != "route" || section.Name != "nas_route" || section.Options["target"] != "10.20.0.0/16" || section.Options["onlink"] != "1" {
		t.Fatalf("unexpected route section: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["metric"].(types.Int64).ValueInt64() != 10 || loaded["mtu"].(types.Int64).ValueInt64() != 1400 || !loaded["onlink"].(types.Bool).ValueBool() {
		t.Fatalf("unexpected route round trip: %+v", loaded)
	}
}

func TestNetworkRouteValidation(t *testing.T) {
	spec := NewNetworkRouteResource().(*typedResource).spec
	base := map[string]attr.Value{
		"name":      types.StringValue("nas_route"),
		"interface": types.StringValue("lan"),
		"target":    types.StringValue("10.20.0.0/16"),
		"gateway":   types.StringValue(""),
		"metric":    types.Int64Value(0),
		"table":     types.StringValue(""),
		"source":    types.StringValue(""),
		"onlink":    types.BoolValue(false),
		"mtu":       types.Int64Value(0),
		"type":      types.StringValue("unicast"),
	}
	for name, mutate := range map[string]func(map[string]attr.Value){
		"missing interface": func(values map[string]attr.Value) { values["interface"] = types.StringValue("") },
		"bad target":        func(values map[string]attr.Value) { values["target"] = types.StringValue("not-a-cidr") },
		"bad gateway":       func(values map[string]attr.Value) { values["gateway"] = types.StringValue("not-an-ip") },
		"negative mtu":      func(values map[string]attr.Value) { values["mtu"] = types.Int64Value(-1) },
	} {
		t.Run(name, func(t *testing.T) {
			values := map[string]attr.Value{}
			for key, value := range base {
				values[key] = value
			}
			mutate(values)
			if err := spec.Validate(values); err == nil {
				t.Fatal("invalid route was accepted")
			}
		})
	}
}
