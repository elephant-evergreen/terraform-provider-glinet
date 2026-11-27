package router

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func configurationFixture() Configuration {
	return Configuration{{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{"ssid": "Example", "mode": "ap", "disabled": "0", ManagedOption: "1"}}, {Package: "firewall", Name: "allow_example", Type: "rule", Options: map[string]string{"target": "ACCEPT", "dest_port": "443", ManagedOption: "1"}}}
}

func TestConfigurationValidation(t *testing.T) {
	if err := ValidateConfiguration(configurationFixture()); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(Configuration){
		"unsupported package": func(c Configuration) { c[0].Package = "dropbear" },
		"secret option":       func(c Configuration) { c[0].Options["key"] = "synthetic-private-value" },
		"invalid identity":    func(c Configuration) { c[0].Name = "wifi2g;reboot" },
		"invalid SSID":        func(c Configuration) { c[0].Options["ssid"] = "invalid\nSSID" },
		"observed drift":      func(c Configuration) { c[0].Missing = true },
		"mismatched selector": func(c Configuration) { c[0].Name = "@device[0]" },
		"absent options":      func(c Configuration) { c[0].Absent = true },
	} {
		t.Run(name, func(t *testing.T) {
			config := configurationFixture()
			mutate(config)
			if err := ValidateConfiguration(config); err == nil {
				t.Fatal("unsafe typed configuration was accepted")
			}
		})
	}
}

func TestConfigurationValidationAcceptsMultipleAnonymousRules(t *testing.T) {
	config := Configuration{
		{Package: "firewall", Name: "@rule[0]", Type: "rule", Match: map[string]string{"name": "first"}, Options: map[string]string{"name": "first", "target": "ACCEPT", ManagedOption: "1"}},
		{Package: "firewall", Name: "@rule[0]", Type: "rule", Match: map[string]string{"name": "second"}, Options: map[string]string{"name": "second", "target": "ACCEPT", ManagedOption: "1"}},
	}
	if err := ValidateConfiguration(config); err != nil {
		t.Fatal(err)
	}
	duplicate := Configuration{
		{Package: "firewall", Name: "rule_first", Type: "rule", Options: map[string]string{"target": "ACCEPT", ManagedOption: "1"}},
		{Package: "firewall", Name: "rule_first", Type: "rule", Options: map[string]string{"target": "ACCEPT", ManagedOption: "1"}},
	}
	if err := ValidateConfiguration(duplicate); err == nil {
		t.Fatal("duplicate named sections must still be rejected")
	}
}

func TestWirelessRadioConfigurationValidation(t *testing.T) {
	radio := Configuration{{Package: "wireless", Name: "mt798611", Type: "wifi-device", Options: map[string]string{"channel": "auto", "htmode": "HE40", "txpower": "100", "country": "JP", "disabled": "0", ManagedOption: "1"}}}
	if err := ValidateConfiguration(radio); err != nil {
		t.Fatal(err)
	}
	iface := Configuration{{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{"ssid": "Example", "mode": "ap", "encryption": "sae", "ieee80211w": "2", "wps_pushbutton": "0", ManagedOption: "1"}}}
	if err := ValidateConfiguration(iface); err != nil {
		t.Fatal(err)
	}
	radio[0].Options["ssid"] = "Example"
	if err := ValidateConfiguration(radio); err == nil {
		t.Fatal("wifi-device accepted a wifi-iface option")
	}
}

const (
	safetyStateMarker   = "GLINET_SAFETY_STATE"
	safetyEnvMarker     = "GLINET_SAFETY_ENV"
	safetyRuntimeMarker = "GLINET_SAFETY_RUNTIME"
)

func safetyStateFixture() []byte {
	return []byte(`{"network":[],"wireless":[],"dhcp":[],"firewall":[],"gl-dns-v2":[],"eth_ports_config_map":[]}`)
}

func safetyEnvironmentFixture() []byte {
	return []byte("ipv6_enabled=0\nipv6_forwarding=0\n@@addresses@@\n\n@@routes@@\n\n@@upnp@@\n0\n\n@@adguard@@\n0\n0\n@@leases@@\n")
}

func safetyRuntimeFixture() []byte {
	return []byte("@@generated@@\niptables -t filter -P INPUT ACCEPT\niptables -t filter -P FORWARD DROP\niptables -t filter -P OUTPUT ACCEPT\niptables -t filter -N reject\niptables -t filter -A FORWARD -j reject\niptables -t filter -A reject -j REJECT\n@@running@@\n*filter\n:INPUT ACCEPT [0:0]\n:FORWARD DROP [0:0]\n:OUTPUT ACCEPT [0:0]\n:reject - [0:0]\n-A FORWARD -j reject\n-A reject -j REJECT\nCOMMIT\n")
}

func safetyTestResponse(script string) ([]byte, bool) {
	switch {
	case strings.Contains(script, safetyStateMarker):
		return safetyStateFixture(), true
	case strings.Contains(script, safetyEnvMarker):
		return safetyEnvironmentFixture(), true
	case strings.Contains(script, safetyRuntimeMarker):
		return safetyRuntimeFixture(), true
	}
	return nil, false
}

type configurationReader struct {
	config Configuration
	calls  int
	err    error
	script string
}

func (r *configurationReader) Run(_ context.Context, script string) ([]byte, error) {
	if data, ok := safetyTestResponse(script); ok {
		return data, nil
	}
	r.calls++
	r.script = script
	if r.err != nil {
		return nil, r.err
	}
	return configurationTestResponse(r.config, script)
}

func TestConfigurationAdoptionDoesNotWriteAndCancellationDoesNotArm(t *testing.T) {
	config := configurationFixture()
	remote := &configurationReader{config: config}
	client := newFixtureClient(remote)
	if err := client.writeConfiguration(context.Background(), config, nil, config, nil); err != nil {
		t.Fatal(err)
	}
	if remote.calls != 1 || strings.Contains(remote.script, "uci commit") {
		t.Fatal("Matching configuration was mutated")
	}
	remote.err = context.Canceled
	if err := client.writeConfiguration(context.Background(), config, nil, config, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if strings.Contains(remote.script, "uci commit") {
		t.Fatal("Canceled read armed recovery")
	}
}

func TestConfigurationGeneratedScriptsAreShellSafe(t *testing.T) {
	config := configurationFixture()
	config[0].Options["ssid"] = "Example ' $(touch /tmp/glinet-should-not-exist)"
	for _, script := range []string{configurationLua(config, false), configurationApplyScript(config, "example-token", reloadPlan{Reboot: true, DNS: true, DHCP: true, Firewall: true}, map[string]string{"wireless.wifi2g.key": "example'password"}), configurationApplyScript(config, "example-token", reloadPlan{DNS: true, DHCP: true, Firewall: true}), configurationConfirmScript("example-token", config)} {
		cmd := exec.Command("/bin/sh", "-n")
		cmd.Stdin = strings.NewReader(script)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Invalid shell: %s %v", out, err)
		}
	}

}

type configurationMutationRunner struct {
	current Configuration
	wanted  Configuration
	apply   string
}

func (r *configurationMutationRunner) Run(_ context.Context, script string) ([]byte, error) {
	if data, ok := safetyTestResponse(script); ok {
		return data, nil
	}
	if strings.Contains(script, "io.write(json.encode(result))") || strings.Contains(script, "keys_match") {
		return configurationTestResponse(r.current, script)
	}
	if strings.Contains(script, "assert(uci:save") {
		r.apply = script
		r.current = r.wanted
	}
	return nil, nil
}
func TestPortMappingOnlyChangeSynchronizesRuntime(t *testing.T) {
	before := Configuration{{Package: "eth_ports_config_map", Name: "lan2", Type: "port", Options: map[string]string{"pvid": "1"}}}
	after := Configuration{{Package: "eth_ports_config_map", Name: "lan2", Type: "port", Options: map[string]string{"pvid": "11"}}}
	remote := &configurationMutationRunner{current: before, wanted: after}
	if err := newFixtureClient(remote).writeConfiguration(context.Background(), after, nil, after, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(remote.apply, "gl-vlan-port-map sync") || !strings.Contains(remote.apply, "reboot") {
		t.Fatal("Port-only mutation skipped runtime synchronization")
	}
}

func configurationTestResponse(config Configuration, script string) ([]byte, error) {
	if strings.Contains(script, "keys_match") {
		return json.Marshal(map[string]any{"configuration": config, "keys_match": true})
	}
	return json.Marshal(config)
}

func TestReadSectionPreservesInvalidObservedSSID(t *testing.T) {
	value := strings.Repeat("x", 1100) + "\n"
	state := uciState{"wireless": {{Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{"ssid": value}}}}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(runnerFunc(func(context.Context, string) ([]byte, error) { return data, nil }), nil)
	actual, found, err := client.ReadSection(context.Background(), Section{Package: "wireless", Name: "wifi2g", Type: "wifi-iface"})
	if err != nil || !found || actual.Options["ssid"] != value {
		t.Fatalf("observed drift became unreadable: %v", err)
	}
}

func TestSSIDChangeReloadsOnlyWireless(t *testing.T) {
	before := Configuration{{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{"ssid": "Before"}}}
	after := Configuration{{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{"ssid": "After"}}}
	remote := &configurationMutationRunner{current: before, wanted: after}
	if err := newFixtureClient(remote).writeConfiguration(context.Background(), after, nil, after, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(remote.apply, "wifi reload") || strings.Contains(remote.apply, "/etc/init.d/network reload") || strings.Contains(remote.apply, "module\\\":\\\"dns") {
		t.Fatal("A wireless setting change reloaded unrelated services")
	}
}

func TestMissingWiFiRequiresExplicitCredentialRestoration(t *testing.T) {
	desired := configurationFixture()
	actual := configurationFixture()
	actual[0].Missing = true
	remote := &configurationReader{config: actual}
	err := newFixtureClient(remote).writeConfiguration(context.Background(), desired, nil, desired, nil)
	if err == nil || !strings.Contains(err.Error(), "write-only key") || remote.calls != 1 {
		t.Fatalf("Missing Wi-Fi was recreated without a key: %v", err)
	}
	script := configurationApplyScript(desired, "token", reloadPlan{})
	if !strings.Contains(script, " enable\nsync\ntransferred=1") || !strings.Contains(script, "touch started\n  sync\n") {
		t.Fatal("Recovery was not persisted before activation and mutation")
	}
}

func TestReadSectionReportsMissingAnonymousIdentity(t *testing.T) {
	client := NewClient(runnerFunc(func(context.Context, string) ([]byte, error) { return []byte(`{"firewall":[]}`), nil }), nil)
	selector := Section{Package: "firewall", Name: "@zone[0]", Type: "zone", Match: map[string]string{"name": "guest"}}
	if _, found, err := client.ReadSection(context.Background(), selector); err != nil || found {
		t.Fatalf("missing anonymous section was not reported: %v", err)
	}
}

type fixtureProtection struct{}

func (fixtureProtection) Protect(context.Context, Runner, bool) error { return nil }

func newFixtureClient(runner Runner) *Client { return NewClient(runner, fixtureProtection{}) }

type rejectingProtection struct{ calls int }

func (p *rejectingProtection) Protect(context.Context, Runner, bool) error {
	p.calls++
	return errors.New("Recovery protection unavailable.")
}

func TestMutationRequiresSuccessfulProtection(t *testing.T) {
	for _, missing := range []bool{true, false} {
		t.Run(map[bool]string{true: "missing", false: "failed"}[missing], func(t *testing.T) {
			before := Configuration{{Package: "dhcp", Name: "example", Type: "host", Options: map[string]string{"ip": "192.168.8.2"}}}
			after := Configuration{{Package: "dhcp", Name: "example", Type: "host", Options: map[string]string{"ip": "192.168.8.3"}}}
			remote := &configurationMutationRunner{current: before, wanted: after}
			protection := &rejectingProtection{}
			client := NewClient(remote, protection)
			if missing {
				client.protection = nil
			}
			err := client.writeConfiguration(context.Background(), after, nil, after, nil)
			if err == nil || remote.apply != "" || !SameConfiguration(remote.current, before) {
				t.Fatalf("mutation bypassed protection: %v", err)
			}
			if !missing && protection.calls != 1 {
				t.Fatal("protection was not called")
			}
		})
	}
}

func TestManagedSectionsAreWritable(t *testing.T) {
	sections := Configuration{
		{Package: "glconfig", Type: "service", Name: "general", Options: map[string]string{"mode": "router", "autotimezone": "1"}, Lists: map[string][]string{"track_ip": {"1.1.1.1"}, "track_ipv6": {"2001:4860:4860::8888"}}},
		{Package: "edgerouter", Type: "global", Name: "global", Options: map[string]string{"enabled": "1", "force_dns": "1"}},
		{Package: "edgerouter", Type: "dhcp", Name: "wandhcp", Options: map[string]string{"leasetime": "43200", "ignore": "1"}},
		{Package: "fullconenat", Type: "global", Name: "global", Options: map[string]string{"enable": "1"}},
		{Package: "gl_dpi", Type: "gl_dpi", Name: "dpi_config", Options: map[string]string{"log": "INFO", "expire_time": "0", "lib_auto_upgrade": "0", "ota_auto_upgrade": "0"}},
		{Package: "gl_dpi_content_protection", Type: "dpi", Name: "content_protection", Options: map[string]string{"enabled": "0"}, Lists: map[string][]string{"category": {"gambling"}}},
		{Package: "gl_led", Type: "led", Name: "global", Options: map[string]string{"led_enable": "1", "led_daemon": "1"}},
		{Package: "mtkhnat", Type: "global", Name: "global", Options: map[string]string{"enable": "1", "hqos": "1", "txq_num": "16", "scheduling": "wrr", "actype": "1"}},
		{Package: "samba4", Type: "sambashare", Name: "media", Options: map[string]string{"name": "media", "path": "/mnt/media", "read_only": "no", "force_root": "0", "timemachine": "0"}},
		{Package: "kmwan", Type: "member", Name: "wan", Options: map[string]string{"interface": "wan", "metric": "10"}, Lists: map[string][]string{"tracks": {"ping,1.1.1.1"}}},
		{Package: "route_policy", Type: "default", Name: "cfg000000", Options: map[string]string{"enabled": "1", "type": "default", "mark": "0x8000"}, Lists: map[string][]string{"from": {"10000"}}},
		{Package: "route_policy", Type: "rule_process", Name: "gl_process", Options: map[string]string{"enabled": "1", "from_type": "process_gid"}, Lists: map[string][]string{"from": {"10000"}}},
	}
	if err := ValidateConfiguration(sections); err != nil {
		t.Fatal(err)
	}
}

func TestAnonymousSectionDeletionRequiresMatch(t *testing.T) {
	match := Configuration{{Package: "firewall", Type: "forwarding", Name: "@forwarding[0]", Match: map[string]string{"src": "lan", "dest": "wan"}, Absent: true}}
	if err := ValidateConfiguration(match); err != nil {
		t.Fatal(err)
	}
	withoutMatch := Configuration{{Package: "firewall", Type: "forwarding", Name: "@forwarding[0]", Absent: true}}
	if err := ValidateConfiguration(withoutMatch); err == nil {
		t.Fatal("anonymous deletion without a match was accepted")
	}
	namedMatch := Configuration{{Package: "firewall", Type: "forwarding", Name: "cfg000000", Match: map[string]string{"src": "lan"}, Absent: true}}
	if err := ValidateConfiguration(namedMatch); err == nil {
		t.Fatal("named deletion with a match was accepted")
	}
}
