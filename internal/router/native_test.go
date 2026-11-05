package router

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
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
	if err := NewClient(transport).Verify(ctx); err != nil {
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
	for _, name := range strings.Fields(configurationPackages) {
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
		output, err := transport.Run(ctx, redirect(configurationLua(config, false, nil)))
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
		if _, err := transport.Run(ctx, redirect(configurationLua(config, true, keys))); err != nil {
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
	_, err = transport.Run(ctx, redirect(configurationLua(duplicate, true, nil)))
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
	_, err = transport.Run(ctx, redirect(configurationLua(missingWiFi, true, nil)))
	if !errors.As(err, &status) || status.Code != 65 || !read(missingWiFi)[0].Missing {
		t.Fatal("Missing Wi-Fi was created without credential restoration")
	}
	write(missingWiFi, map[string]string{"missing_ap": "fixture-secret-restored"})
	if !SameConfiguration(read(missingWiFi), missingWiFi) {
		t.Fatal("Explicit Wi-Fi credential restoration did not converge")
	}
	check := "test \"$(uci -c " + quote(conf) + " -P " + quote(saved) + " get wireless.wifi2g.key)\" = 'fixture-secret-updated'\n"
	if _, err := transport.Run(ctx, check); err != nil {
		t.Fatal("Write-only fixture key was not stored")
	}
}
