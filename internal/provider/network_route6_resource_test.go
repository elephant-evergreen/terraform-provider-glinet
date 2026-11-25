package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestNetworkRoute6SpecRoundTrip(t *testing.T) {
	spec := NewNetworkRoute6Resource().(*typedResource).spec
	values := map[string]attr.Value{
		"name": types.StringValue("nas_route6"), "interface": types.StringValue("lan"),
		"target": types.StringValue("2001:db8:20::/48"), "gateway": types.StringValue("2001:db8:1::1"),
		"metric": types.Int64Value(10), "table": types.StringValue(""), "source": types.StringValue(""),
		"onlink": types.BoolValue(false), "mtu": types.Int64Value(1400), "type": types.StringValue("unicast"),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Type != "route6" || section.Options["target"] != "2001:db8:20::/48" {
		t.Fatalf("unexpected route6: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["metric"].(types.Int64).ValueInt64() != 10 {
		t.Fatalf("unexpected round trip: %+v", loaded)
	}
}
