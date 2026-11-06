package router

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

type runnerFunc func(context.Context, string) ([]byte, error)

func (f runnerFunc) Run(ctx context.Context, script string) ([]byte, error) { return f(ctx, script) }

func TestValidationAndShellQuoting(t *testing.T) {
	for _, value := range []string{"", "@wifi-iface[0]", "foo;true", "../wireless", strings.Repeat("x", 65)} {
		if ValidSection(value) {
			t.Fatalf("Accepted invalid section %q", value)
		}
	}
	for _, value := range []string{"", "bad\nSSID", "bad\x00SSID", strings.Repeat("あ", 11)} {
		if ValidSSID(value) {
			t.Fatalf("Accepted invalid SSID %q", value)
		}
	}
	value := `a'$(printf injected);"b`
	cmd := exec.Command("/bin/sh", "-c", "printf '%s' "+quote(value))
	actual, err := cmd.Output()
	if err != nil || string(actual) != value {
		t.Fatalf("Shell interpolation changed the value: %q, %v", actual, err)
	}
	config := Configuration{{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{"ssid": value, "mode": "ap"}}}
	script := configurationApplyScript(config, strings.Repeat("a", 32), reloadPlan{Wireless: true}, map[string]string{"wireless.wifi2g.key": "example'password"})
	cmd = exec.Command("/bin/sh", "-n")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Invalid shell syntax: %v: %s", err, output)
	}
}
