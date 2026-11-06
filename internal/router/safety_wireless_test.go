package router

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestWirelessOnlyConfigurationReadsSafetyState(t *testing.T) {
	config := Configuration{{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{
		"ifname": "rax0", "network": "lan", "ssid": "Example", "mode": "ap",
	}}}
	state := `{"wireless":[{"name":"stock_guest","type":"wifi-iface","options":{"ifname":"rax0","network":"lan"}}]}`
	requested := 0
	client := NewClient(runnerFunc(func(_ context.Context, script string) ([]byte, error) {
		if !strings.Contains(script, "GLINET_SAFETY_STATE") {
			return nil, errors.New("unexpected script")
		}
		requested++
		return []byte(state), nil
	}))
	err := client.checkConfigurationSafety(context.Background(), config)
	if requested == 0 {
		t.Fatal("wireless-only configuration did not read the safety state")
	}
	if err == nil || !strings.Contains(err.Error(), "Unmanaged Wi-Fi section stock_guest") {
		t.Fatalf("wireless-only configuration did not reject the unmanaged radio conflict: %v", err)
	}
}

func TestWirelessOnlyConfigurationWithoutConflictIsAccepted(t *testing.T) {
	config := Configuration{{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{
		"ifname": "rax0", "network": "lan", "ssid": "Example", "mode": "ap",
	}}}
	state := `{"wireless":[{"name":"wifi2g","type":"wifi-iface","options":{"ifname":"rax0","network":"lan"}}]}`
	client := NewClient(runnerFunc(func(_ context.Context, _ string) ([]byte, error) {
		return []byte(state), nil
	}))
	if err := client.checkConfigurationSafety(context.Background(), config); err != nil {
		t.Fatalf("managed wireless section was rejected: %v", err)
	}
}
