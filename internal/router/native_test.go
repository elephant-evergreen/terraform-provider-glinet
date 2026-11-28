package router

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNativeUCIFixture(t *testing.T) {
	host := os.Getenv("GLINET_NATIVE_TEST_HOST")
	if host == "" {
		t.Skip("Set GLINET_NATIVE_TEST_HOST only for an explicitly authorized native UCI fixture test")
	}
	transport, err := NewSSH(host, 22, os.Getenv("GLINET_NATIVE_TEST_PRIVATE_KEY_FILE"), os.Getenv("GLINET_NATIVE_TEST_KNOWN_HOSTS_FILE"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := newFixtureClient(transport).Verify(ctx); err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	root := "/tmp/terraform-glinet-native-" + hex.EncodeToString(nonce)
	conf := root + "/config"
	saved := root + "/saved"
	setup := "set -eu\numask 077\nmkdir " + quote(root) + "\nmkdir " + quote(conf) + " " + quote(saved) + "\n"
	packages := map[string]bool{}
	for _, name := range strings.Fields(configurationPackages) {
		packages[name] = true
	}
	if start := strings.Index(safetyStateLua, "local packages={"); start >= 0 {
		if end := strings.Index(safetyStateLua[start:], "}"); end >= 0 {
			for _, match := range regexp.MustCompile(`'([a-z0-9_-]+)'`).FindAllStringSubmatch(safetyStateLua[start:start+end], -1) {
				packages[match[1]] = true
			}
		}
	}
	for name := range packages {
		setup += "touch " + quote(conf+"/"+name) + "\n"
	}
	firewall := "config rule 'stock_allow'\n option name 'Stock Allow'\n option target 'ACCEPT'\nconfig rule 'managed_a'\n option name 'Managed A'\n option target 'REJECT'\nconfig rule 'stock_other'\n option name 'Stock Other'\n option target 'ACCEPT'\nconfig rule 'managed_b'\n option name 'Managed B'\n option target 'REJECT'\n"
	wireless := "config wifi-iface 'wifi2g'\n option mode 'ap'\n option ssid 'Example'\n option key 'fixture-secret-original'\n"
	setup += "printf '%s' " + quote(firewall) + " > " + quote(conf+"/firewall") + "\nprintf '%s' " + quote(wireless) + " > " + quote(conf+"/wireless") + "\n"
	if _, err := transport.Run(ctx, setup); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		if _, err := transport.Run(cleanup, "rm -rf "+quote(root)+"\ntest ! -e "+quote(root)+"\n"); err != nil {
			t.Error("Native fixture cleanup failed:", err)
		}
	})
	redirect := func(script string) string {
		return strings.Replace(script, "require('uci').cursor()", "require('uci').cursor("+luaQuote(conf)+","+luaQuote(saved)+")", 1)
	}
	read := func(config Configuration) Configuration {
		t.Helper()
		output, err := transport.Run(ctx, redirect(configurationLua(config, false)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(output), "fixture-secret") {
			t.Fatal("Native observation returned a Wi-Fi secret")
		}
		var actual Configuration
		if json.Unmarshal(output, &actual) != nil {
			t.Fatal("Invalid native fixture observation")
		}
		for i := range actual {
			if len(actual[i].Options) == 0 {
				actual[i].Options = nil
			}
			if len(actual[i].Lists) == 0 {
				actual[i].Lists = nil
			}
		}
		return actual
	}
	write := func(config Configuration, keys map[string]string) {
		t.Helper()
		secrets := map[string]string{}
		for name, value := range keys {
			secrets["wireless."+name+".key"] = value
		}
		if _, err := transport.Run(ctx, redirect(configurationLua(config, true, secrets))); err != nil {
			t.Fatal(err)
		}
	}
	order := func() string {
		t.Helper()
		script := "lua - <<'LUA'\nlocal c=require('uci').cursor(" + luaQuote(conf) + "," + luaQuote(saved) + ")\nc:foreach('firewall',nil,function(s) io.write(s['.name']..'\\n') end)\nLUA\n"
		out, err := transport.Run(ctx, script)
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}
	rules := Configuration{{Package: "firewall", Name: "managed_a", Type: "rule", Options: map[string]string{"name": "Updated A"}}, {Package: "firewall", Name: "managed_b", Type: "rule", Options: map[string]string{"name": "Updated B"}}}
	original := order()
	write(rules, nil)
	if order() != original {
		t.Fatal("Unspecified positions moved rules relative to unmanaged rules")
	}
	a, b := 3, 1
	rules[0].Position = &a
	rules[1].Position = &b
	write(rules, nil)
	if order() != "stock_allow\nmanaged_b\nstock_other\nmanaged_a\n" {
		t.Fatal("Explicit positions did not preserve unmanaged rule order")
	}
	if !SameConfiguration(read(rules), rules) {
		t.Fatal("Positioned rules did not converge")
	}
	anonymous := Configuration{{Package: "firewall", Name: "@forwarding[0]", Type: "forwarding", Match: map[string]string{"src": "guest", "dest": "wan"}, Options: map[string]string{"enabled": "1"}}}
	if !read(anonymous)[0].Missing {
		t.Fatal("Missing anonymous section was not observed")
	}
	write(anonymous, nil)
	if !SameConfiguration(read(anonymous), anonymous) {
		t.Fatal("Anonymous identity was not recreated")
	}
	duplicate := append(Configuration{}, anonymous...)
	copySection := anonymous[0]
	copySection.Name = "@forwarding[1]"
	duplicate = append(duplicate, copySection)
	_, err = transport.Run(ctx, redirect(configurationLua(duplicate, true)))
	var status *ExitError
	if !errors.As(err, &status) || status.Code != 65 {
		t.Fatal("Duplicate physical identities were accepted")
	}
	dns := Configuration{{Package: "dhcp", Name: "@dnsmasq[0]", Type: "dnsmasq", Options: map[string]string{"noresolv": "1"}, Lists: map[string][]string{"server": {"127.0.0.1#5453"}}}}
	write(dns, nil)
	if !SameConfiguration(read(dns), dns) {
		t.Fatal("Native list or missing singleton handling failed")
	}
	missingRules := Configuration{{Package: "firewall", Name: "new_a", Type: "rule", Options: map[string]string{"target": "REJECT"}}, {Package: "firewall", Name: "new_b", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
	missing := read(missingRules)
	if !missing[0].Missing || !missing[1].Missing || missing[0].Name != "new_a" || missing[1].Name != "new_b" {
		t.Fatal("Missing rules lost their declared relative order")
	}
	write(missingRules, nil)
	if !SameConfiguration(read(missingRules), missingRules) {
		t.Fatal("Multiple missing rules did not converge")
	}
	wifi := Configuration{{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{"ssid": "Example Updated"}}}
	write(wifi, map[string]string{"wifi2g": "fixture-secret-updated"})
	if !SameConfiguration(read(wifi), wifi) {
		t.Fatal("Native AP setting handling failed")
	}
	missingWiFi := Configuration{{Package: "wireless", Name: "missing_ap", Type: "wifi-iface", Options: map[string]string{"mode": "ap", "encryption": "psk2", "ssid": "Example Restored"}}}
	_, err = transport.Run(ctx, redirect(configurationLua(missingWiFi, true)))
	if !errors.As(err, &status) || status.Code != 65 || !read(missingWiFi)[0].Missing {
		t.Fatal("Missing Wi-Fi was created without credential restoration")
	}
	write(missingWiFi, map[string]string{"missing_ap": "fixture-secret-restored"})
	if !SameConfiguration(read(missingWiFi), missingWiFi) {
		t.Fatal("Explicit Wi-Fi credential restoration did not converge")
	}
	optional := Configuration{{Package: "network", Name: "example", Type: "interface", Options: map[string]string{"proto": "static", "gateway": "192.168.8.1"}, Lists: map[string][]string{"dns": {"192.168.8.1"}}}}
	write(optional, nil)
	optional[0].Options["gateway"] = ""
	optional[0].Lists["dns"] = nil
	write(optional, nil)
	if !SameConfiguration(read(optional), optional) {
		t.Fatal("Cleared scalar/list attributes did not converge in native UCI")
	}
	if _, err := transport.Run(ctx, "test -z \"$(uci -c "+quote(conf)+" -P "+quote(saved)+" -q get network.example.gateway || true)\"\ntest -z \"$(uci -c "+quote(conf)+" -P "+quote(saved)+" -q get network.example.dns || true)\"\n"); err != nil {
		t.Fatal("Optional attributes remain in native UCI")
	}
	check := "test \"$(uci -c " + quote(conf) + " -P " + quote(saved) + " get wireless.wifi2g.key)\" = 'fixture-secret-updated'\n"
	if _, err := transport.Run(ctx, check); err != nil {
		t.Fatal("Write-only fixture key was not stored")
	}
}

func TestNativeSafetyStateExcludesSecrets(t *testing.T) {
	host := os.Getenv("GLINET_NATIVE_TEST_HOST")
	if host == "" {
		t.Skip("Set GLINET_NATIVE_TEST_HOST only for an explicitly authorized native UCI fixture test")
	}
	transport, err := NewSSH(host, 22, os.Getenv("GLINET_NATIVE_TEST_PRIVATE_KEY_FILE"), os.Getenv("GLINET_NATIVE_TEST_KNOWN_HOSTS_FILE"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	root := "/tmp/terraform-glinet-safety-" + hex.EncodeToString(nonce)
	conf := root + "/config"
	saved := root + "/saved"
	setup := "set -eu\numask 077\nmkdir " + quote(root) + "\nmkdir " + quote(conf) + " " + quote(saved) + "\n"
	for _, name := range strings.Fields(configurationPackages) {
		setup += "touch " + quote(conf+"/"+name) + "\n"
	}
	network := "config interface 'pppoe'\n option proto 'pppoe'\n option username 'fixture-user'\n option password 'fixture-pppoe-secret'\nconfig wireguard_vpn 'peer'\n option public_key 'fixture-public-key'\n"
	wireless := "config wifi-iface 'wifi2g'\n option mode 'ap'\n option ssid 'Example'\n option key 'fixture-wifi-secret'\n option wpa_psk 'fixture-wpa-secret'\n"
	setup += "printf '%s' " + quote(network) + " > " + quote(conf+"/network") + "\nprintf '%s' " + quote(wireless) + " > " + quote(conf+"/wireless") + "\n"
	if _, err := transport.Run(ctx, setup); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		if _, err := transport.Run(cleanup, "rm -rf "+quote(root)+"\ntest ! -e "+quote(root)+"\n"); err != nil {
			t.Error("Native safety fixture cleanup failed:", err)
		}
	})
	script := "lua - <<'GLINET_SAFETY_STATE'\n" + strings.Replace(safetyStateLua, "require('uci').cursor()", "require('uci').cursor("+luaQuote(conf)+","+luaQuote(saved)+")", 1) + "\nGLINET_SAFETY_STATE\n"
	var buffer strings.Builder
	if err := transport.RunTo(ctx, script, &buffer); err != nil {
		t.Fatalf("Native safety state failed: %v: %s", err, buffer.String())
	}
	output := []byte(buffer.String())
	for _, secret := range []string{"fixture-pppoe-secret", "fixture-wifi-secret", "fixture-wpa-secret"} {
		if strings.Contains(string(output), secret) {
			t.Fatalf("Native safety state returned the secret %q", secret)
		}
	}
	for _, kept := range []string{"fixture-user", "fixture-public-key"} {
		if !strings.Contains(string(output), kept) {
			t.Fatalf("Native safety state dropped the non-secret option %q", kept)
		}
	}
}
