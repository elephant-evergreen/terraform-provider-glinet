package provider

import (
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestDependentResourcesApplyInOrder(t *testing.T) {
	remote := &typedFixtureRunner{}
	client := router.NewClient(remote, fixtureProtection{})
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := `
resource "glinet_firewall_zone" "acceptance" {
  delete_behavior = "delete"
  name            = "acceptance_zone"
  input           = "REJECT"
}

resource "glinet_firewall_rules" "acceptance" {
  rules = [{
    name       = "acceptance-rule"
    src        = glinet_firewall_zone.acceptance.name
    dest       = "wan"
    protocols  = ["tcp"]
    dest_ports = ["65530"]
    target     = "REJECT"
  }]
}
`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config: config,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("glinet_firewall_zone.acceptance", "name", "acceptance_zone"),
				resource.TestCheckResourceAttr("glinet_firewall_rules.acceptance", "rules.0.name", "acceptance-rule"),
				resource.TestCheckResourceAttr("glinet_firewall_rules.acceptance", "rules.0.src", "acceptance_zone"),
			),
		}},
	})
}
