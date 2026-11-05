package router

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

type runnerFunc func(context.Context, string) ([]byte, error)

func (f runnerFunc) Run(ctx context.Context, script string) ([]byte, error) { return f(ctx, script) }

func TestReadSettingsAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		output string
		err    error
		want   error
		valid  bool
	}{
		{name: "AP", output: "Example Wi-Fi\x000\x000\x001\x00", valid: true},
		{name: "missing", err: &ExitError{Code: 44}, want: ErrNotFound},
		{name: "firmware", err: &ExitError{Code: 65}},
		{name: "bad flag", output: "SSID\x002\x000\x000\x00"},
		{name: "extra field", output: "SSID\x000\x000\x000\x00secret\x00"},
		{name: "malformed", output: "SSID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient(runnerFunc(func(_ context.Context, script string) ([]byte, error) {
				if strings.Contains(script, ".key") || strings.Contains(script, "uci export") {
					t.Fatal("Read must never request passwords or complete configuration")
				}
				return []byte(tc.output), tc.err
			}))
			got, err := client.Read(context.Background(), "wifi2g")
			if tc.valid {
				if err != nil || got.SSID != "Example Wi-Fi" || got.Disabled || got.Hidden || !got.Isolate {
					t.Fatalf("Read: %+v, %v", got, err)
				}
			} else if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("Expected a typed error, got %v", err)
			}
		})
	}
}

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
		t.Fatalf("Shell interpolation changed the SSID: %q, %v", actual, err)
	}
	for _, script := range []string{applyScript(Settings{Section: "wifi2g", SSID: value}, strings.Repeat("a", 32)), confirmScript(strings.Repeat("a", 32))} {
		cmd := exec.Command("/bin/sh", "-n")
		cmd.Stdin = strings.NewReader(script)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Invalid shell syntax: %v: %s", err, output)
		}
	}
}

func TestWriteNoopAndPreflightFailure(t *testing.T) {
	calls := 0
	client := NewClient(runnerFunc(func(context.Context, string) ([]byte, error) {
		calls++
		return []byte("Example\x000\x000\x001\x00"), nil
	}))
	if err := client.Write(context.Background(), Settings{Section: "wifi2g", SSID: "Example", Isolate: true}); err != nil || calls != 1 {
		t.Fatalf("Unchanged settings should not write: %v, calls=%d", err, calls)
	}
	calls = 0
	client = NewClient(runnerFunc(func(context.Context, string) ([]byte, error) { calls++; return nil, &ExitError{Code: 65} }))
	if err := client.Write(context.Background(), Settings{Section: "wifi2g", SSID: "New"}); err == nil || calls != 1 {
		t.Fatalf("Unsupported firmware must stop before writes: %v, calls=%d", err, calls)
	}
}
