package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestConfigureGuardsUnknownAndExplicitEmptySettings(t *testing.T) {
	for _, name := range []string{"GLINET_HOST", "GLINET_PRIVATE_KEY_FILE", "GLINET_KNOWN_HOSTS_FILE"} {
		t.Setenv(name, "fixture-value")
	}
	for _, tc := range []struct {
		name string
		host interface{}
		want string
	}{
		{name: "unknown", host: tftypes.UnknownValue, want: "Unknown connection setting"},
		{name: "explicit empty", host: "", want: "Missing connection setting"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := New("test")()
			var schema provider.SchemaResponse
			p.Schema(context.Background(), provider.SchemaRequest{}, &schema)
			config := tfsdk.Config{Schema: schema.Schema, Raw: tftypes.NewValue(schema.Schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
				"host": tftypes.NewValue(tftypes.String, tc.host), "port": tftypes.NewValue(tftypes.Number, nil),
				"private_key_file": tftypes.NewValue(tftypes.String, nil), "known_hosts_file": tftypes.NewValue(tftypes.String, nil),
				"backup_directory": tftypes.NewValue(tftypes.String, nil),
			})}
			var response provider.ConfigureResponse
			p.Configure(context.Background(), provider.ConfigureRequest{Config: config}, &response)
			if !response.Diagnostics.HasError() || !strings.Contains(response.Diagnostics.Errors()[0].Summary(), tc.want) {
				t.Fatalf("Unexpected configuration result: %v", response.Diagnostics)
			}
		})
	}
}

type fixtureProvider struct {
	provider.Provider
	client *router.Client
}

func (p *fixtureProvider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	resp.ResourceData = p.client
}
