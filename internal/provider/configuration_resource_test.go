package provider

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func configurationSafetyResponse(script string) ([]byte, bool) {
	switch {
	case strings.Contains(script, "GLINET_SAFETY_STATE"):
		return []byte(`{"network":[],"wireless":[],"dhcp":[],"firewall":[],"gl-dns-v2":[],"eth_ports_config_map":[]}`), true
	case strings.Contains(script, "GLINET_SAFETY_ENV"):
		return []byte("ipv6_enabled=0\nipv6_forwarding=0\n@@addresses@@\n\n@@routes@@\n\n@@upnp@@\n0\n\n@@adguard@@\n0\n0\n@@leases@@\n"), true
	case strings.Contains(script, "GLINET_SAFETY_RUNTIME"):
		return []byte("@@generated@@\niptables -t filter -P INPUT ACCEPT\niptables -t filter -P FORWARD DROP\niptables -t filter -N reject\niptables -t filter -A FORWARD -j reject\niptables -t filter -A reject -j REJECT\n@@running@@\n*filter\n:INPUT ACCEPT [0:0]\n:FORWARD DROP [0:0]\n:reject - [0:0]\n-A FORWARD -j reject\n-A reject -j REJECT\nCOMMIT\n"), true
	}
	return nil, false
}

type configurationRunner struct {
	mu        sync.Mutex
	actual    router.Configuration
	writes    int
	pending   bool
	keyWrites int
}

func (r *configurationRunner) Run(_ context.Context, script string) ([]byte, error) {
	if data, ok := configurationSafetyResponse(script); ok {
		return data, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.Contains(script, "keys_match") {
		if r.pending {
			return nil, &router.ExitError{Code: 73}
		}
		return json.Marshal(map[string]any{"configuration": r.actual, "keys_match": !strings.Contains(script, "changed-fixture-password")})
	}
	if strings.Contains(script, "io.write(json.encode(result))") {
		return json.Marshal(r.actual)
	}
	if strings.Contains(script, "assert(uci:save") {
		match := regexp.MustCompile(`local spec=json.decode\('((?:\\.|[^'])*)'\)`).FindStringSubmatch(script)
		if len(match) != 2 {
			panic("Missing fixture configuration")
		}
		data := strings.NewReplacer(`\\`, `\`, `\'`, `'`).Replace(match[1])
		var next router.Configuration
		if json.Unmarshal([]byte(data), &next) != nil {
			panic("Invalid fixture configuration")
		}
		r.actual = next
		r.writes++
		if strings.Contains(script, "assert(uci:set('wireless',") {
			r.keyWrites++
		}
	}
	return nil, nil
}
func TestConfigurationProtocolAdoptionUpdateDriftAndForget(t *testing.T) {
	initial := router.Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"name": "Example", "target": "REJECT"}}}
	remote := &configurationRunner{actual: initial}
	client := router.NewClient(remote)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := func(label string) string {
		return `resource "glinet_uci_configuration" "test" {
 configuration=jsonencode([{package="firewall",name="example",type="rule",options={name="` + label + `",target="REJECT"}}])
 wireless_keys_version=1
}`
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			remote.mu.Lock()
			defer remote.mu.Unlock()
			if remote.writes != 2 || remote.actual[0].Options["name"] != "Updated" {
				t.Fatalf("Unexpected writes or destroy mutation: %d", remote.writes)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config("Example"), Check: func(*terraform.State) error {
				if remote.writes != 0 {
					t.Fatal("Adoption mutated the router")
				}
				return nil
			}},
			{Config: config("Updated")},
			{PreConfig: func() { remote.mu.Lock(); remote.pending = true; remote.mu.Unlock() }, Config: config("Updated"), PlanOnly: true, ExpectError: regexp.MustCompile("Another change or recovery is pending")},
			{PreConfig: func() { remote.mu.Lock(); remote.pending = false; remote.mu.Unlock() }, Config: config("Updated")},
			{PreConfig: func() { remote.mu.Lock(); remote.actual[0].Options["name"] = "Outside"; remote.mu.Unlock() }, Config: config("Updated"), PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: config("Updated")},
		}})
}

func TestConfigurationPartialDriftDoesNotRotateUnchangedGeneration(t *testing.T) {
	remote := &configurationRunner{actual: router.Configuration{
		{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}},
		{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{"ssid": "Example"}},
	}}
	client := router.NewClient(remote)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := func(password string) string {
		return `resource "glinet_uci_configuration" "test" {
 configuration=jsonencode([{package="firewall",name="example",type="rule",options={target="REJECT"}},{package="wireless",name="wifi2g",type="wifi-iface",options={ssid="Example"}}])
 wireless_keys_wo={wifi2g="` + password + `"}
 wireless_keys_version=1
}`
	}
	check := func(state *terraform.State) error {
		data, _ := json.Marshal(state)
		if strings.Contains(string(data), "fixture-password") {
			t.Fatal("Write-only value entered Terraform state")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			remote.mu.Lock()
			defer remote.mu.Unlock()
			if remote.keyWrites != 0 || remote.writes != 1 {
				t.Fatalf("Partial drift reset key lifecycle: writes=%d keys=%d", remote.writes, remote.keyWrites)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config("initial-fixture-password"), Check: check},
			{PreConfig: func() { remote.mu.Lock(); remote.actual[0].Missing = true; remote.mu.Unlock() }, Config: config("changed-fixture-password"), Check: check},
		}})
}
