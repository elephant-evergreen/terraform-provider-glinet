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

type typedFixtureRunner struct {
	mu         sync.Mutex
	actual     router.Configuration
	applyCalls int
}

func (r *typedFixtureRunner) Run(_ context.Context, script string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.Contains(script, "GLINET_SAFETY_STATE") {
		return safetyStateJSON(r.actual), nil
	}
	if strings.Contains(script, "GLINET_SAFETY_ENV") || strings.Contains(script, "GLINET_SAFETY_RUNTIME") {
		if data, ok := configurationSafetyResponse(script); ok {
			return data, nil
		}
		return nil, nil
	}
	if strings.Contains(script, "assert(uci:save") {
		change, err := typedChangeConfiguration(script)
		if err != nil {
			return nil, err
		}
		for _, desired := range change {
			updated := r.actual[:0]
			for _, existing := range r.actual {
				if !typedSameIdentity(existing, desired) {
					updated = append(updated, existing)
				}
			}
			r.actual = append(updated, desired)
		}
		r.applyCalls++
		return nil, nil
	}
	if strings.Contains(script, "keys_match") || strings.Contains(script, "io.write(json.encode(result))") {
		change, err := typedChangeConfiguration(script)
		if err != nil {
			return nil, err
		}
		observed := router.Configuration{}
		for _, desired := range change {
			found := false
			for _, existing := range r.actual {
				if typedSameIdentity(existing, desired) {
					observed = append(observed, existing)
					found = true
					break
				}
			}
			if !found {
				desired.Missing = true
				observed = append(observed, desired)
			}
		}
		if strings.Contains(script, "keys_match") {
			return json.Marshal(map[string]any{"configuration": observed, "keys_match": true})
		}
		return json.Marshal(observed)
	}
	return nil, nil
}

func typedSameIdentity(a, b router.Section) bool {
	if a.Package != b.Package || a.Type != b.Type {
		return false
	}
	if b.Name != "" && !strings.HasPrefix(b.Name, "@") {
		return a.Name == b.Name
	}
	if len(b.Match) > 0 {
		for key, value := range b.Match {
			if a.Options[key] != value {
				return false
			}
		}
		return true
	}
	return a.Name == b.Name
}

func typedChangeConfiguration(script string) (router.Configuration, error) {
	match := regexp.MustCompile(`local spec=json.decode\('((?:\\.|[^'])*)'\)`).FindStringSubmatch(script)
	if len(match) != 2 {
		return nil, context.Canceled
	}
	data := strings.NewReplacer(`\\`, `\`, `\'`, `'`).Replace(match[1])
	var config router.Configuration
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return nil, err
	}
	return config, nil
}

func safetyStateJSON(config router.Configuration) []byte {
	state := map[string][]map[string]any{}
	for _, section := range config {
		item := map[string]any{"name": section.Name, "type": section.Type, "options": section.Options, "lists": section.Lists}
		state[section.Package] = append(state[section.Package], item)
	}
	encoded, _ := json.Marshal(state)
	return encoded
}

func TestTypedNetworkInterfaceProtocol(t *testing.T) {
	remote := &typedFixtureRunner{}
	client := router.NewClient(remote)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := func(address string) string {
		return `resource "glinet_network_interface" "test" {
  name = "guest"
  device = "br-guest"
  address = "` + address + `"
}`
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: config("192.168.9.1/24"), Check: func(state *terraform.State) error {
				if remote.applyCalls != 1 {
					t.Fatalf("apply calls = %d", remote.applyCalls)
				}
				return nil
			}},
			{Config: config("192.168.9.2/24")},
			{ResourceName: "glinet_network_interface.test", ImportState: true, ImportStateVerify: true},
		}})
}

func configurationSafetyResponse(script string) ([]byte, bool) {
	switch {
	case strings.Contains(script, "GLINET_SAFETY_ENV"):
		return []byte("ipv6_enabled=0\nipv6_forwarding=0\n@@addresses@@\n\n@@routes@@\n\n@@upnp@@\n0\n\n@@adguard@@\n0\n0\n@@leases@@\n"), true
	case strings.Contains(script, "GLINET_SAFETY_RUNTIME"):
		return []byte("@@generated@@\niptables -t filter -P INPUT ACCEPT\niptables -t filter -P FORWARD DROP\niptables -t filter -N reject\niptables -t filter -A FORWARD -j reject\niptables -t filter -A reject -j REJECT\n@@running@@\n*filter\n:INPUT ACCEPT [0:0]\n:FORWARD DROP [0:0]\n:reject - [0:0]\n-A FORWARD -j reject\n-A reject -j REJECT\nCOMMIT\n"), true
	}
	return nil, false
}
