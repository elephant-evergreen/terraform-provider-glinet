package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestPreservedAttributesAreOptionalAndComputed(t *testing.T) {
	ctx := context.Background()
	for _, factory := range New("test")().Resources(ctx) {
		typed, ok := factory().(*typedResource)
		if !ok {
			continue
		}
		var response frameworkresource.SchemaResponse
		typed.Schema(ctx, frameworkresource.SchemaRequest{}, &response)
		for _, name := range typed.spec.PreserveUnset {
			attribute, ok := response.Schema.Attributes[name]
			if !ok {
				t.Fatalf("%s preserves missing attribute %q", typed.spec.TypeName, name)
			}
			if !attribute.IsOptional() || !attribute.IsComputed() {
				t.Fatalf("%s preserves attribute %q that is not optional and computed", typed.spec.TypeName, name)
			}
		}
	}
}

func TestTypedInterfaceKeepsUndeclaredOptions(t *testing.T) {
	remote := &typedFixtureRunner{}
	client := router.NewClient(remote, fixtureProtection{})
	client.AllowNetworkDisruption(true)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := func(address, ip6assign string) string {
		line := ""
		if ip6assign != "" {
			line = "\n  ip6assign = " + ip6assign
		}
		return `resource "glinet_network_interface" "test" {
  name  = "guest"
  device = "br-guest"
  address = "` + address + `"` + line + `
  delete_behavior = "delete"
}`
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: config("192.168.9.1/24", "60")},
			{Config: config("192.168.9.1/24", "62"), Check: resource.TestCheckResourceAttr("glinet_network_interface.test", "ip6assign", "62")},
			{Config: config("192.168.9.2/24", ""), Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("glinet_network_interface.test", "address", "192.168.9.2/24"),
				resource.TestCheckResourceAttr("glinet_network_interface.test", "ip6assign", "62"),
			)},
		}})
}

func TestDHCPReservationLoadReadsAdoptedOptions(t *testing.T) {
	resource := NewDHCPReservationResource().(*typedResource)
	values := resource.spec.Load(router.Section{
		Package: "dhcp", Type: "host", Name: "legacy_host",
		Options: map[string]string{
			"mac": "02:00:00:00:00:02", "ip": "192.168.9.20", "name": "legacy",
			"duid": "00:01:00:01:00:00:00:00", "leasetime": "12h", "ignore": "1",
			"instance": "lan", "tag": "guest", "set_tag": "fixed", "broadcast": "1",
		},
	})
	for name, want := range map[string]string{
		"duid": "00:01:00:01:00:00:00:00", "leasetime": "12h", "instance": "lan", "tag": "guest", "set_tag": "fixed",
	} {
		if got, ok := values[name].(types.String); !ok || got.ValueString() != want {
			t.Fatalf("%s = %+v, want %q", name, values[name], want)
		}
	}
	if got, ok := values["broadcast"].(types.Bool); !ok || !got.ValueBool() {
		t.Fatalf("broadcast = %+v, want true", values["broadcast"])
	}
	if got, ok := values["ignore"].(types.Bool); !ok || !got.ValueBool() {
		t.Fatalf("ignore = %+v, want true", values["ignore"])
	}
}

func TestTypedBridgeWritesMacaddr(t *testing.T) {
	remote := &typedFixtureRunner{}
	client := router.NewClient(remote, fixtureProtection{})
	client.AllowNetworkDisruption(true)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := `resource "glinet_network_bridge" "test" {
  name = "br-test"
  empty = true
  macaddr = "02:00:00:00:00:03"
  delete_behavior = "delete"
}`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: config, Check: func(*terraform.State) error {
				remote.mu.Lock()
				defer remote.mu.Unlock()
				for _, section := range remote.actual {
					if section.Package == "network" && section.Options["name"] == "br-test" {
						if section.Options["macaddr"] != "02:00:00:00:00:03" {
							return fmt.Errorf("bridge macaddr was not written: %q", section.Options["macaddr"])
						}
						return nil
					}
				}
				return fmt.Errorf("bridge section was not created")
			}},
		}})
}
