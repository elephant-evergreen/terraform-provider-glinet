package provider

import (
	"context"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestTypedLoadUsesDeclaredAttributes(t *testing.T) {
	ctx := context.Background()
	for _, factory := range New("test")().Resources(ctx) {
		typed, ok := factory().(*typedResource)
		if !ok {
			continue
		}
		var response frameworkresource.SchemaResponse
		typed.Schema(ctx, frameworkresource.SchemaRequest{}, &response)
		for name := range typed.spec.Load(router.Section{}) {
			if _, ok := response.Schema.Attributes[name]; !ok {
				t.Fatalf("%s loads undeclared attribute %q", typed.spec.TypeName, name)
			}
		}
	}
}

func TestSelectorsKeepOptionsOutsideTheSchema(t *testing.T) {
	cases := []struct {
		resource   *typedResource
		values     map[string]attr.Value
		unexpected []string
	}{
		{
			resource: NewMiniDLNAResource().(*typedResource),
			values: map[string]attr.Value{
				"enabled": types.BoolValue(false), "port": types.Int64Value(8200),
				"interface": types.StringValue("br-lan"), "friendly_name": types.StringValue("DLNA"),
				"media_dir": stringSet([]string{"/mnt"}),
			},
			unexpected: []string{"inotify", "user", "db_dir", "notify_interval", "root_container"},
		},
		{
			resource: NewSystemSSHResource().(*typedResource),
			values: map[string]attr.Value{
				"enabled": types.BoolValue(true), "port": types.Int64Value(22),
				"password_auth": types.BoolValue(true), "root_password_auth": types.BoolValue(true),
				"interface": types.StringValue(""), "idle_timeout": types.Int64Value(0),
			},
			unexpected: []string{"Banner", "GatewayPorts", "MaxAuthTries"},
		},
		{
			resource: NewAdminAccessResource().(*typedResource),
			values: map[string]attr.Value{
				"enabled": types.BoolValue(true), "listen_http": stringSet(nil), "listen_https": stringSet(nil),
				"redirect_https": types.BoolValue(false), "rfc1918_filter": types.BoolValue(true),
				"max_requests": types.Int64Value(3), "max_connections": types.Int64Value(100), "no_dirlists": types.BoolValue(true),
			},
			unexpected: []string{"home", "cert", "key", "tcp_keepalive", "http_keepalive"},
		},
	}
	for _, tc := range cases {
		section, err := tc.resource.spec.Selector(tc.values)
		if err != nil {
			t.Fatalf("%s selector: %v", tc.resource.spec.TypeName, err)
		}
		for _, name := range tc.unexpected {
			if _, ok := section.Options[name]; ok {
				t.Fatalf("%s writes undeclared option %q", tc.resource.spec.TypeName, name)
			}
		}
	}
}

func TestSingletonSectionIdentities(t *testing.T) {
	cases := []struct {
		resource *typedResource
		pkg      string
		kind     string
		name     string
	}{
		{NewSystemNTPResource().(*typedResource), "system", "timeserver", "ntp"},
		{NewAdGuardResource().(*typedResource), "adguardhome", "adguardhome", "config"},
	}
	for _, tc := range cases {
		section, err := tc.resource.spec.Selector(map[string]attr.Value{})
		if err != nil {
			t.Fatalf("%s selector: %v", tc.resource.spec.TypeName, err)
		}
		read, err := tc.resource.spec.ReadSelector(map[string]attr.Value{})
		if err != nil {
			t.Fatalf("%s read selector: %v", tc.resource.spec.TypeName, err)
		}
		for _, got := range []router.Section{section, read} {
			if got.Package != tc.pkg || got.Type != tc.kind || got.Name != tc.name {
				t.Fatalf("%s identifies %s.%s.%s, want %s.%s.%s", tc.resource.spec.TypeName, got.Package, got.Type, got.Name, tc.pkg, tc.kind, tc.name)
			}
		}
	}
}
