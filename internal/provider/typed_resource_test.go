package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type typedFixtureRunner struct {
	mu              sync.Mutex
	actual          router.Configuration
	applyCalls      int
	rejectOwnership bool
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
		if r.rejectOwnership {
			return nil, &router.ExitError{Code: 78}
		}
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
			r.actual = updated
			if !desired.Absent {
				desired.Options = withoutEmptyOptions(desired.Options)
				r.actual = append(r.actual, desired)
			}
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
					if desired.Absent {
						desired.Absent = false
						observed = append(observed, desired)
					} else {
						observed = append(observed, readSection(existing, desired))
					}
					found = true
					break
				}
			}
			if !found {
				if !desired.Absent {
					desired.Missing = true
				}
				observed = append(observed, desired)
			}
		}
		if strings.Contains(script, "keys_match") {
			keysMatch := true
			for _, section := range observed {
				if section.Missing {
					keysMatch = false
				}
			}
			return json.Marshal(map[string]any{"configuration": observed, "keys_match": keysMatch})
		}
		return json.Marshal(observed)
	}
	return nil, nil
}

func readSection(existing, desired router.Section) router.Section {
	section := existing
	section.Options = map[string]string{}
	for key, value := range existing.Options {
		section.Options[key] = value
	}
	section.Lists = map[string][]string{}
	for key := range desired.Options {
		if _, ok := section.Options[key]; !ok {
			section.Options[key] = ""
		}
	}
	for key := range desired.Lists {
		section.Lists[key] = existing.Lists[key]
	}
	if len(section.Lists) == 0 {
		section.Lists = nil
	}
	return section
}

func withoutEmptyOptions(options map[string]string) map[string]string {
	result := make(map[string]string, len(options))
	for key, value := range options {
		if value != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
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
	client := router.NewClient(remote, fixtureProtection{})
	client.AllowNetworkDisruption(true)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := func(address string) string {
		return `resource "glinet_network_interface" "test" {
  name = "guest"
  device = "br-guest"
  address = "` + address + `"
  delete_behavior = "delete"
}`
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			if len(remote.actual) != 0 {
				t.Fatal("Destroy preserved a delete-managed section")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config("192.168.9.1/24") + `
resource "glinet_dhcp_reservation" "optional" {
 section="example"
 mac="02:00:00:00:00:01"
 ip=cidrhost(glinet_network_interface.test.address,10)
 hostname="old-host"
 dns=["192.168.9.1"]
 delete_behavior="delete"
}
`, Check: func(state *terraform.State) error {
				if remote.applyCalls != 2 {
					t.Fatalf("apply calls = %d", remote.applyCalls)
				}
				return nil
			}},
			{Config: config("192.168.9.2/24") + `
resource "glinet_dhcp_reservation" "optional" {
 section="example"
 mac="02:00:00:00:00:01"
 ip=cidrhost(glinet_network_interface.test.address,10)
 delete_behavior="delete"
}
`},
			{Config: config("192.168.9.2/24"), PreConfig: func() {
				remote.mu.Lock()
				defer remote.mu.Unlock()
				for _, s := range remote.actual {
					if s.Package == "network" {
						s.Options["ipaddr"] = "192.168.9.99"
					}
				}
			}, Check: resource.TestCheckResourceAttr("glinet_network_interface.test", "address", "192.168.9.2/24")},
			{Config: config("192.168.9.2/24"), PreConfig: func() { remote.mu.Lock(); defer remote.mu.Unlock(); remote.actual = nil }},
			{ResourceName: "glinet_network_interface.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"delete_behavior"}},
		}})
}

func configurationSafetyResponse(script string) ([]byte, bool) {
	switch {
	case strings.Contains(script, "GLINET_SAFETY_ENV"):
		return []byte("ipv6_enabled=0\nipv6_forwarding=0\n@@addresses@@\n\n@@routes@@\n\n@@upnp@@\n0\n\n@@adguard@@\n0\n0\n@@leases@@\n"), true
	case strings.Contains(script, "GLINET_SAFETY_RUNTIME"):
		return []byte("@@generated@@\niptables -t filter -P INPUT ACCEPT\niptables -t filter -P FORWARD DROP\niptables -t filter -P OUTPUT ACCEPT\niptables -t filter -N reject\niptables -t filter -A FORWARD -j reject\niptables -t filter -A reject -j REJECT\n@@running@@\n*filter\n:INPUT ACCEPT [0:0]\n:FORWARD DROP [0:0]\n:OUTPUT ACCEPT [0:0]\n:reject - [0:0]\n-A FORWARD -j reject\n-A reject -j REJECT\nCOMMIT\n"), true
	}
	return nil, false
}

func TestPreserveWriteOnlyVersion(t *testing.T) {
	r := &typedResource{spec: typedSpec{WriteOnly: []string{"key_wo"}}}
	values := map[string]attr.Value{"key_version": int64Value(1)}
	r.preserveWriteOnlyVersions(map[string]attr.Value{"key_version": int64Value(3)}, values)
	if values["key_version"].(types.Int64).ValueInt64() != 3 {
		t.Fatalf("key_version was overwritten: %+v", values["key_version"])
	}
	values = map[string]attr.Value{"key_version": int64Value(1)}
	r.preserveWriteOnlyVersions(map[string]attr.Value{"key_version": types.Int64Null()}, values)
	if values["key_version"].(types.Int64).ValueInt64() != 1 {
		t.Fatalf("null key_version should keep the loaded default: %+v", values["key_version"])
	}
}

type fixtureProtection struct{}

func (fixtureProtection) Protect(context.Context, router.Runner, bool) error { return nil }

func TestTypedWirelessInterfaceProtocol(t *testing.T) {
	remote := &typedFixtureRunner{}
	client := router.NewClient(remote, fixtureProtection{})
	client.AllowNetworkDisruption(true)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := func(encryption string) string {
		return `resource "glinet_wireless_interface" "test" {
  section = "wifi2g"
  ssid = "Example"
  device = "mt798611"
  network = "lan"
  mode = "ap"
  encryption = "` + encryption + `"
  key_wo = "example-passphrase"
  key_version = 1
}`
	}
	checkOptions := func(encryption, pmf string) func(*terraform.State) error {
		return func(*terraform.State) error {
			remote.mu.Lock()
			defer remote.mu.Unlock()
			for _, section := range remote.actual {
				if section.Package != "wireless" || section.Name != "wifi2g" {
					continue
				}
				if section.Options["encryption"] != encryption {
					t.Fatalf("encryption = %q", section.Options["encryption"])
				}
				if section.Options["ieee80211w"] != pmf {
					t.Fatalf("ieee80211w = %q", section.Options["ieee80211w"])
				}
				if section.Options["wps_pushbutton"] != "0" {
					t.Fatalf("wps_pushbutton = %q", section.Options["wps_pushbutton"])
				}
				return nil
			}
			t.Fatal("wireless section was not applied")
			return nil
		}
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: config("sae"), Check: checkOptions("sae", "2")},
			{Config: config("psk2"), Check: checkOptions("psk2", "0")},
			{ResourceName: "glinet_wireless_interface.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"key_wo", "key_version", "delete_behavior"}},
		}})
}

func TestTypedWirelessRadioProtocol(t *testing.T) {
	remote := &typedFixtureRunner{actual: router.Configuration{{Package: "wireless", Name: "mt798611", Type: "wifi-device", Options: map[string]string{"band": "2g", "channel": "6", "htmode": "HE40", "txpower": "80", "country": "JP", "disabled": "0"}}}}
	client := router.NewClient(remote, fixtureProtection{})
	client.AllowNetworkDisruption(true)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := `resource "glinet_wireless_radio" "test" {
  device = "mt798611"
  channel = "11"
  htmode = "HE40"
}`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			remote.mu.Lock()
			defer remote.mu.Unlock()
			for _, section := range remote.actual {
				if section.Package == "wireless" && section.Name == "mt798611" {
					return nil
				}
			}
			t.Fatal("Destroy removed the radio section")
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config, Check: func(*terraform.State) error {
				remote.mu.Lock()
				defer remote.mu.Unlock()
				for _, section := range remote.actual {
					if section.Package != "wireless" || section.Name != "mt798611" {
						continue
					}
					if section.Options["channel"] != "11" || section.Options["htmode"] != "HE40" {
						t.Fatalf("radio options = %+v", section.Options)
					}
					return nil
				}
				t.Fatal("radio section was not applied")
				return nil
			}},
			{Config: config, PlanOnly: true},
		}})
}

func TestTypedWirelessRadioRequiresExisting(t *testing.T) {
	remote := &typedFixtureRunner{}
	client := router.NewClient(remote, fixtureProtection{})
	client.AllowNetworkDisruption(true)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: `resource "glinet_wireless_radio" "test" {
  device = "mt798611"
  channel = "auto"
}`, ExpectError: regexp.MustCompile("Section not found")},
		}})
}

func TestDeleteRequiresOwnershipMarker(t *testing.T) {
	for _, test := range []struct {
		name    string
		options map[string]string
		want    bool
	}{
		{"managed", map[string]string{router.ManagedOption: "1"}, true},
		{"unmanaged", map[string]string{}, false},
		{"released", map[string]string{router.ManagedOption: "0"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := router.IsManaged(test.options); got != test.want {
				t.Fatalf("IsManaged(%v) = %v, want %v", test.options, got, test.want)
			}
		})
	}
}

func TestReleasedSectionIsNotWritable(t *testing.T) {
	if !releasedFromManagement(router.Section{Options: map[string]string{router.ManagedOption: "0"}}) {
		t.Fatal("released marker was not detected")
	}
	for _, options := range []map[string]string{{router.ManagedOption: "1"}, {}} {
		if releasedFromManagement(router.Section{Options: options}) {
			t.Fatalf("managed marker %v was treated as released", options)
		}
	}
}

func TestOwnerConflict(t *testing.T) {
	for _, test := range []struct {
		routerOwner, stateOwner string
		conflict                bool
	}{
		{"", "", false},
		{"a", "", false},
		{"", "a", false},
		{"a", "a", false},
		{"a", "b", true},
	} {
		if got := ownerConflict(test.routerOwner, test.stateOwner); got != test.conflict {
			t.Errorf("ownerConflict(%q, %q) = %v, want %v", test.routerOwner, test.stateOwner, got, test.conflict)
		}
	}
}

func TestTypedCreateRecordsOwnerToken(t *testing.T) {
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
	var token string
	check := func(*terraform.State) error {
		for _, section := range remote.actual {
			if section.Name != "guest" {
				continue
			}
			if !router.IsManaged(section.Options) {
				return fmt.Errorf("guest is not marked managed")
			}
			owner := router.ManagedOwner(section.Options)
			if owner == "" {
				return fmt.Errorf("guest has no owner token")
			}
			if token == "" {
				token = owner
			} else if owner != token {
				return fmt.Errorf("owner token changed from %q to %q", token, owner)
			}
			return nil
		}
		return fmt.Errorf("guest section was not recorded")
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: config, Check: check},
			{Config: config, Check: check},
		},
	})
}

func TestTypedCreateRefusesForeignOwner(t *testing.T) {
	remote := &typedFixtureRunner{actual: router.Configuration{{
		Package: "network", Type: "interface", Name: "guest",
		Options: map[string]string{"device": "br-guest", "proto": "static", "ipaddr": "192.168.9.1", "netmask": "255.255.255.0", router.ManagedOption: "1", router.OwnerTokenOption: "foreign"},
	}}}
	client := router.NewClient(remote, fixtureProtection{})
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := `resource "glinet_network_interface" "test" {
  name            = "guest"
  device          = "br-guest"
  address         = "192.168.9.1/24"
  delete_behavior = "delete"
}`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{Config: config, ExpectError: regexp.MustCompile("owned by another configuration")}},
	})
}

func TestTypedForgetClearsOwnerTokenOnRouter(t *testing.T) {
	remote := &typedFixtureRunner{}
	client := router.NewClient(remote, fixtureProtection{})
	client.AllowNetworkDisruption(true)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := `resource "glinet_network_interface" "test" {
  name            = "guest"
  device          = "br-guest"
  address         = "192.168.9.1/24"
  delete_behavior = "forget"
}`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{Config: config, Check: func(*terraform.State) error {
			remote.mu.Lock()
			defer remote.mu.Unlock()
			for _, section := range remote.actual {
				if section.Package == "network" && section.Name == "guest" {
					if router.ManagedOwner(section.Options) == "" {
						return fmt.Errorf("created section has no owner token")
					}
				}
			}
			return nil
		}}},
		CheckDestroy: func(*terraform.State) error {
			remote.mu.Lock()
			defer remote.mu.Unlock()
			for _, section := range remote.actual {
				if section.Package != "network" || section.Name != "guest" {
					continue
				}
				if router.IsManaged(section.Options) {
					return fmt.Errorf("forget kept the managed marker")
				}
				if token, ok := section.Options[router.OwnerTokenOption]; ok && token != "" {
					return fmt.Errorf("forget kept the owner token %q", token)
				}
				return nil
			}
			return fmt.Errorf("forget removed the section")
		},
	})
}

func TestTypedWriteReportsConcurrentOwnership(t *testing.T) {
	remote := &typedFixtureRunner{rejectOwnership: true}
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
		Steps: []resource.TestStep{{Config: config, ExpectError: regexp.MustCompile("owned by another Terraform state")}},
	})
}
