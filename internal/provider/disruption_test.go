package provider

import (
	"regexp"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestResolveAllowNetworkDisruption(t *testing.T) {
	t.Setenv("GLINET_ALLOW_NETWORK_DISRUPTION", "")
	if allow, err := resolveAllowNetworkDisruption(types.BoolNull()); err != nil || allow {
		t.Fatalf("default = %v, %v", allow, err)
	}
	if allow, err := resolveAllowNetworkDisruption(types.BoolValue(true)); err != nil || !allow {
		t.Fatalf("configured true = %v, %v", allow, err)
	}
	t.Setenv("GLINET_ALLOW_NETWORK_DISRUPTION", "true")
	if allow, err := resolveAllowNetworkDisruption(types.BoolNull()); err != nil || !allow {
		t.Fatalf("environment true = %v, %v", allow, err)
	}
	t.Setenv("GLINET_ALLOW_NETWORK_DISRUPTION", "maybe")
	if _, err := resolveAllowNetworkDisruption(types.BoolNull()); err == nil {
		t.Fatal("invalid environment value was accepted")
	}
	if _, err := resolveAllowNetworkDisruption(types.BoolUnknown()); err == nil {
		t.Fatal("unknown value was accepted")
	}
}

func TestNetworkDisruptionRequiresPermission(t *testing.T) {
	remote := &typedFixtureRunner{}
	client := router.NewClient(remote, fixtureProtection{})
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := `resource "glinet_network_interface" "test" {
  name            = "guest"
  device          = "br-guest"
  address         = "192.168.9.1/24"
  delete_behavior = "delete"
}`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{Config: config, ExpectError: regexp.MustCompile("can interrupt the network")}},
	})
}

func TestNetworkDisruptionAllowed(t *testing.T) {
	remote := &typedFixtureRunner{}
	client := router.NewClient(remote, fixtureProtection{})
	client.AllowNetworkDisruption(true)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := `resource "glinet_network_interface" "test" {
  name            = "guest"
  device          = "br-guest"
  address         = "192.168.9.1/24"
  delete_behavior = "delete"
}`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{Config: config}},
	})
}
