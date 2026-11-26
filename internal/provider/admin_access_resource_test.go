package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestAdminAccessSpecRoundTrip(t *testing.T) {
	spec := NewAdminAccessResource().(*typedResource).spec
	values := map[string]attr.Value{"enabled": types.BoolValue(true), "listen_http": types.StringValue("0.0.0.0:80"), "listen_https": types.StringValue("0.0.0.0:443"), "redirect_https": types.BoolValue(true), "rfc1918_filter": types.BoolValue(true), "max_requests": types.Int64Value(3), "max_connections": types.Int64Value(100), "no_dirlists": types.BoolValue(true)}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "uhttpd" || section.Options["enabled"] != "1" {
		t.Fatalf("unexpected admin access: %+v", section)
	}
}
