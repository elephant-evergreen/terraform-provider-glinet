package provider

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type fixtureRunner struct {
	mu       sync.Mutex
	settings router.Settings
	exists   bool
	writes   int
}

func (f *fixtureRunner) Run(_ context.Context, script string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.exists {
		return nil, &router.ExitError{Code: 44}
	}
	if strings.Contains(script, "uci set ") {
		for _, match := range regexp.MustCompile(`uci set 'wireless\.wifi2g\.([a-z]+)=([^']*)'`).FindAllStringSubmatch(script, -1) {
			switch match[1] {
			case "ssid":
				f.settings.SSID = match[2]
			case "disabled":
				f.settings.Disabled = match[2] == "1"
			case "hidden":
				f.settings.Hidden = match[2] == "1"
			case "isolate":
				f.settings.Isolate = match[2] == "1"
			}
		}
		f.writes++
	}
	flag := func(value bool) string {
		if value {
			return "1"
		}
		return "0"
	}
	if strings.Contains(script, "printf '%s\\000'") {
		return []byte(f.settings.SSID + "\x00" + flag(f.settings.Disabled) + "\x00" + flag(f.settings.Hidden) + "\x00" + flag(f.settings.Isolate) + "\x00"), nil
	}
	return nil, nil
}

type fixtureProvider struct {
	provider.Provider
	client *router.Client
}

func (p *fixtureProvider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	resp.ResourceData = p.client
}

func TestProtocolLifecycleImportDriftAndForget(t *testing.T) {
	remote := &fixtureRunner{settings: router.Settings{Section: "wifi2g", SSID: "Original"}, exists: true}
	client := router.NewClient(remote)
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client}),
	}
	config := func(ssid string, hidden bool) string {
		value := "false"
		if hidden {
			value = "true"
		}
		return `resource "glinet_wireless_interface_settings" "test" {
  section = "wifi2g"
  ssid = "` + ssid + `"
  disabled = false
  hidden = ` + value + `
  isolate = true
}`
	}
	resource.Test(t, resource.TestCase{
		IsUnitTest: true, ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			remote.mu.Lock()
			defer remote.mu.Unlock()
			if !remote.exists || remote.settings.SSID != "Updated" || !remote.settings.Hidden || !remote.settings.Isolate || remote.writes != 3 {
				t.Fatalf("Destroy changed settings or lifecycle missed a write: %+v, writes=%d", remote.settings, remote.writes)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config("First", false), Check: resource.TestCheckResourceAttr("glinet_wireless_interface_settings.test", "ssid", "First")},
			{Config: config("Updated", true), Check: resource.TestCheckResourceAttr("glinet_wireless_interface_settings.test", "hidden", "true")},
			{ResourceName: "glinet_wireless_interface_settings.test", ImportState: true, ImportStateId: "wifi2g", ImportStateVerify: true, ImportStateVerifyIdentifierAttribute: "section"},
			{PreConfig: func() { remote.mu.Lock(); remote.settings.SSID = "Outside"; remote.mu.Unlock() }, Config: config("Updated", true), PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: config("Updated", true), Check: resource.TestCheckResourceAttr("glinet_wireless_interface_settings.test", "ssid", "Updated")},
		},
	})
}

func TestReadMissingSectionRemovesState(t *testing.T) {
	r := &wirelessSettingsResource{client: router.NewClient(&fixtureRunner{exists: false})}
	var schema frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &schema)
	state := tfsdk.State{Schema: schema.Schema}
	data := wirelessSettingsModel{Section: types.StringValue("wifi2g"), SSID: types.StringValue("Before"), Disabled: types.BoolValue(false), Hidden: types.BoolValue(false), Isolate: types.BoolValue(true)}
	if diagnostics := state.Set(context.Background(), &data); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	response := frameworkresource.ReadResponse{State: state}
	r.Read(context.Background(), frameworkresource.ReadRequest{State: state}, &response)
	if response.Diagnostics.HasError() || !response.State.Raw.IsNull() {
		t.Fatalf("Missing section did not leave state cleanly: %v", response.Diagnostics)
	}
}

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
			})}
			var response provider.ConfigureResponse
			p.Configure(context.Background(), provider.ConfigureRequest{Config: config}, &response)
			if !response.Diagnostics.HasError() || !strings.Contains(response.Diagnostics.Errors()[0].Summary(), tc.want) {
				t.Fatalf("Unexpected configuration result: %v", response.Diagnostics)
			}
		})
	}
}
