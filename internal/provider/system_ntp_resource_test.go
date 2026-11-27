package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSystemNTPSpecRoundTrip(t *testing.T) {
	spec := NewSystemNTPResource().(*typedResource).spec
	values := map[string]attr.Value{"enabled": types.BoolValue(true), "servers": stringSet([]string{"pool.ntp.org"}), "use_dhcp": types.BoolValue(false)}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "system" || section.Type != "timeserver" || section.Name != "ntp" || section.Lists["server"][0] != "pool.ntp.org" {
		t.Fatalf("unexpected ntp section: %+v", section)
	}
}
