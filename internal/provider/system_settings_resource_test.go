package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSystemSettingsSpecRoundTrip(t *testing.T) {
	spec := NewSystemSettingsResource().(*typedResource).spec
	values := map[string]attr.Value{"hostname": types.StringValue("flint2"), "timezone": types.StringValue("Asia/Tokyo"), "zonename": types.StringValue("JST")}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "system" || section.Type != "system" || section.Options["timezone"] != "Asia/Tokyo" {
		t.Fatalf("unexpected system section: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["hostname"].(types.String).ValueString() != "flint2" {
		t.Fatalf("unexpected round trip: %+v", loaded)
	}
}
