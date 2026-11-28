package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

func TestSSHManagementAdvisory(t *testing.T) {
	plan := func(enabled bool, port int64) map[string]attr.Value {
		return map[string]attr.Value{"enabled": types.BoolValue(enabled), "port": types.Int64Value(port)}
	}
	for _, test := range []struct {
		name           string
		plan           map[string]attr.Value
		state          map[string]attr.Value
		creating       bool
		managementPort int
		want           bool
	}{
		{"unchanged endpoint", plan(true, 22), plan(true, 22), false, 22, false},
		{"port moved away from the management path", plan(true, 2222), plan(true, 22), false, 22, true},
		{"port change beyond an unknown management port", plan(true, 2222), plan(true, 22), false, 0, true},
		{"creation on the management port", plan(true, 2222), nil, true, 2222, false},
		{"creation on another port", plan(true, 22), nil, true, 2222, true},
		{"disabled", plan(false, 22), plan(true, 22), false, 22, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := sshManagementAdvisory(test.plan, test.state, test.creating, test.managementPort)
			if (got != "") != test.want {
				t.Fatalf("advisory = %q, want warning = %v", got, test.want)
			}
		})
	}
	if NewSystemSSHResource().(*typedResource).spec.Advisory == nil {
		t.Fatal("the SSH resource does not publish its management-path advisory")
	}
}
