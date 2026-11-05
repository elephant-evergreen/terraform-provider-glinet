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
		strings.Replace(valid, `"ssid"`, `"api_token"`, 1),
		strings.Replace(valid, `"wifi2g"`, `"wifi2g;reboot"`, 1),
		strings.Replace(valid, `"options":`, `"unknown":`, 1),
		valid + ` {}`, `[]`,
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

type configurationReader struct {
	config Configuration
	calls  int
	err    error
	script string
}

func (r *configurationReader) Run(_ context.Context, script string) ([]byte, error) {
	r.calls++
	r.script = script
	if r.err != nil {
		return nil, r.err
	}
	return json.Marshal(r.config)
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
	for _, script := range []string{configurationLua(config, false, nil), configurationApplyScript(config, map[string]string{"wifi2g": "example'password"}, "example-token", true), configurationApplyScript(config, nil, "example-token", false), configurationConfirmScript("example-token", config, nil)} {
		cmd := exec.Command("/bin/sh", "-n")
		cmd.Stdin = strings.NewReader(script)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Invalid shell: %s %v", out, err)
		}
	}
	script := configurationApplyScript(config, nil, "token", true)
	if strings.Index(script, "armed=1") > strings.Index(script, "assert(uci:save") || !strings.Contains(script, "START=99") || !strings.Contains(script, "/decision") {
		t.Fatal("Recovery is not armed before changes, persistent, or atomically decided")
	}
}
