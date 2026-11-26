package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestSystemSSHSpecRoundTrip(t *testing.T) {
	spec := NewSystemSSHResource().(*typedResource).spec
	values := map[string]attr.Value{"enabled": types.BoolValue(true), "port": types.Int64Value(22), "password_auth": types.BoolValue(true), "root_password_auth": types.BoolValue(false), "interface": types.StringValue("")}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "dropbear" || section.Options["RootPasswordAuth"] != "0" {
		t.Fatalf("unexpected SSH: %+v", section)
	}
}
