package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

func TestSSHEndpointWritable(t *testing.T) {
	values := func(enabled bool, port int64) map[string]attr.Value {
		return map[string]attr.Value{"enabled": types.BoolValue(enabled), "port": types.Int64Value(port)}
	}
	client := router.NewClient(nil, nil)
	client.SetManagementPort(22)
	for _, test := range []struct {
		name    string
		values  map[string]attr.Value
		allowed bool
	}{
		{"matching endpoint", values(true, 22), true},
		{"different port", values(true, 2222), false},
		{"disabled", values(false, 22), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := sshEndpointWritable(client, test.values)
			if (err == nil) != test.allowed {
				t.Fatalf("sshEndpointWritable = %v, want allowed = %v", err, test.allowed)
			}
		})
	}
	if err := sshEndpointWritable(nil, values(true, 2222)); err != nil {
		t.Fatalf("an unconfigured client rejected the endpoint: %v", err)
	}
	if NewSystemSSHResource().(*typedResource).spec.AuthorizeWrite == nil {
		t.Fatal("the SSH resource does not authorize endpoint changes")
	}
}

func TestTypedSystemSSHRefusesEndpointChange(t *testing.T) {
	remote := &typedFixtureRunner{}
	client := router.NewClient(remote, fixtureProtection{})
	client.AllowNetworkDisruption(true)
	client.SetManagementPort(22)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: `resource "glinet_system_ssh" "test" {
  port = 22
}`, Check: func(*terraform.State) error {
				remote.mu.Lock()
				defer remote.mu.Unlock()
				for _, script := range remote.applies {
					if !strings.Contains(script, "/etc/init.d/dropbear reload") {
						return fmt.Errorf("SSH activation is missing from the transaction")
					}
				}
				return nil
			}},
			{Config: `resource "glinet_system_ssh" "test" {
  port = 2222
}`, ExpectError: regexp.MustCompile("does not match the provider's management port")},
			{Config: `resource "glinet_system_ssh" "test" {
  enabled = false
}`, ExpectError: regexp.MustCompile("disabling SSH would remove the provider's management path")},
		},
	})
}
