package provider

import (
	"context"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type fakeGoodCloudClient struct {
	enabled    bool
	running    bool
	registered bool
	applied    []bool
	forgot     int
}

func (f *fakeGoodCloudClient) Cloud(context.Context) (router.CloudState, error) {
	return router.CloudState{Enabled: f.enabled, Running: f.running, Registered: f.registered}, nil
}

func (f *fakeGoodCloudClient) ApplyCloud(_ context.Context, enabled bool) error {
	f.enabled = enabled
	f.running = enabled
	f.applied = append(f.applied, enabled)
	return nil
}

func (f *fakeGoodCloudClient) ForgetCloud(context.Context) error {
	f.forgot++
	return nil
}

type goodCloudFixtureProvider struct {
	provider.Provider
	client goodCloudClient
}

func (p *goodCloudFixtureProvider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	resp.ResourceData = p.client
}

func TestGoodCloudResourceProtocol(t *testing.T) {
	client := &fakeGoodCloudClient{}
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"glinet": providerserver.NewProtocol6WithError(&goodCloudFixtureProvider{Provider: New("test")(), client: client}),
	}
	config := func(enabled bool) string {
		value := "false"
		if enabled {
			value = "true"
		}
		return `resource "glinet_goodcloud" "test" {
  enabled = ` + value + `
}
`
	}
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			if client.forgot != 1 {
				t.Fatalf("ForgetCloud calls = %d, want 1", client.forgot)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: config(false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("glinet_goodcloud.test", "enabled", "false"),
					resource.TestCheckResourceAttr("glinet_goodcloud.test", "id", "goodcloud"),
				),
			},
			{
				Config: config(true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("glinet_goodcloud.test", "enabled", "true"),
					resource.TestCheckResourceAttr("glinet_goodcloud.test", "id", "goodcloud"),
				),
			},
			{
				ResourceName:      "glinet_goodcloud.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
