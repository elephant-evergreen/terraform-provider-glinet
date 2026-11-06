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
	return Configuration{{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{"ssid": "Example", "mode": "ap", "disabled": "0"}}, {Package: "firewall", Name: "allow_example", Type: "rule", Options: map[string]string{"target": "ACCEPT", "dest_port": "443"}}}
}

func TestConfigurationValidation(t *testing.T) {
	valid := configurationJSON(configurationFixture())
	if _, err := DecodeConfiguration(valid); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(valid, `"wireless"`, `"dropbear"`, 1),
		strings.Replace(valid, `"ssid"`, `"key"`, 1),
		strings.Replace(valid, `"ssid"`, `"key1"`, 1),
		`[{"package":"network","name":"lan","type":"interface","match":{"device":"br-lan"}}]`,
		strings.Replace(valid, `"ssid"`, `"api_token"`, 1),
		strings.Replace(valid, `"wifi2g"`, `"wifi2g;reboot"`, 1),
		strings.Replace(valid, `"options":`, `"unknown":`, 1),
		valid + ` {}`, `[]`,
		`[{"package":"wireless","name":"wifi2g","type":"wifi-iface","lists":{"network":["lan"]}}]`,
		`[{"package":"network","name":"@interface[0]","type":"device"}]`,
		`[{"package":"network","name":"lan","type":"interface","lists":{"ports":[]}}]`,
		`[{"package":"wireless","name":"wifi2g","type":"wifi-iface","absent":true,"options":{"ssid":"x"}}]`,
	} {
		if _, err := DecodeConfiguration(bad); err == nil {
			t.Fatalf("Accepted invalid configuration: %s", bad)
		}
	}
	if ValidWirelessKeys(configurationFixture(), map[string]string{"missing": "example-password"}) {
		t.Fatal("Key for unmanaged section accepted")
	}
	if !ValidWirelessKeys(configurationFixture(), map[string]string{"wifi2g": "example-password"}) {
		t.Fatal("Valid write-only key rejected")
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
	return []byte("@@generated@@\niptables -t filter -P INPUT ACCEPT\niptables -t filter -P FORWARD DROP\niptables -t filter -N reject\niptables -t filter -A FORWARD -j reject\niptables -t filter -A reject -j REJECT\n@@running@@\n*filter\n:INPUT ACCEPT [0:0]\n:FORWARD DROP [0:0]\n:reject - [0:0]\n-A FORWARD -j reject\n-A reject -j REJECT\nCOMMIT\n")
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
	client := NewClient(remote)
	if err := client.WriteConfiguration(context.Background(), config, nil); err != nil {
		t.Fatal(err)
	}
	if remote.calls != 1 || strings.Contains(remote.script, "uci commit") {
		t.Fatal("Matching configuration was mutated")
	}
	remote.err = context.Canceled
	if err := client.WriteConfiguration(context.Background(), config, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if strings.Contains(remote.script, "uci commit") {
		t.Fatal("Canceled read armed recovery")
	}
}

func TestConfigurationGeneratedScriptsAreShellSafe(t *testing.T) {
	config := configurationFixture()
	config[0].Options["ssid"] = "Example ' $(touch /tmp/glinet-should-not-exist)"
	for _, script := range []string{configurationLua(config, false, nil), configurationApplyScript(config, map[string]string{"wifi2g": "example'password"}, "example-token", reloadPlan{Reboot: true, DNS: true, DHCP: true, Firewall: true}), configurationApplyScript(config, nil, "example-token", reloadPlan{DNS: true, DHCP: true, Firewall: true}), configurationConfirmScript("example-token", config, nil)} {
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
	if err := NewClient(remote).WriteConfiguration(context.Background(), after, nil); err != nil {
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

func TestObservedDriftCanBeReadAgainWithoutApplyingDesiredValidation(t *testing.T) {
	config := configurationFixture()
	config[0].Options["ssid"] = strings.Repeat("x", 1100) + "\n"
	config[0].Missing = true
	encoded := configurationJSON(config)
	if _, err := DecodeConfiguration(encoded); err == nil {
		t.Fatal("Invalid desired configuration was accepted")
	}
	observed, err := DecodeObservedConfiguration(encoded)
	if err != nil || !SameConfiguration(observed, config) {
		t.Fatalf("Observed drift became unreadable: %v", err)
	}
}

func TestSSIDChangeReloadsOnlyWireless(t *testing.T) {
	before := Configuration{{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{"ssid": "Before"}}}
	after := Configuration{{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{"ssid": "After"}}}
	remote := &configurationMutationRunner{current: before, wanted: after}
	if err := NewClient(remote).WriteConfiguration(context.Background(), after, nil); err != nil {
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
	err := NewClient(remote).WriteConfiguration(context.Background(), desired, nil)
	if err == nil || !strings.Contains(err.Error(), "write-only key") || remote.calls != 1 {
		t.Fatalf("Missing Wi-Fi was recreated without a key: %v", err)
	}
	script := configurationApplyScript(desired, nil, "token", reloadPlan{})
	if !strings.Contains(script, " enable\nsync\ntransferred=1") || !strings.Contains(script, "touch started\n  sync\n") {
		t.Fatal("Recovery was not persisted before activation and mutation")
	}
}

func TestMissingAnonymousIdentityObservationRemainsReadable(t *testing.T) {
	config := Configuration{{Package: "firewall", Name: "@zone[0]", Type: "zone", Match: map[string]string{"name": "guest"}, Options: map[string]string{"name": ""}, Missing: true}}
	for range 2 {
		observed, err := DecodeObservedConfiguration(configurationJSON(config))
		if err != nil || !SameConfiguration(observed, config) {
			t.Fatalf("Missing anonymous observation could not be refreshed: %v", err)
		}
		config = observed
	}
	if _, err := DecodeConfiguration(configurationJSON(config)); err == nil {
		t.Fatal("Invalid desired identity accepted")
	}
	script := configurationApplyScript(configurationFixture(), nil, "token", reloadPlan{})
	if !strings.Contains(script, "ln recover decision 2>/dev/null || true\nsync\ncase") {
		t.Fatal("Rollback can run before the winning decision is persisted")
	}
}
