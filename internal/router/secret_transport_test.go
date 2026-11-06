package router

import (
	"strings"
	"testing"
)

func TestSecretTransportIsSeparateFromDesiredOptions(t *testing.T) {
	config := Configuration{{Package: "network", Name: "wg0", Type: "interface", Options: map[string]string{"proto": "wireguard"}}}
	secrets := map[string]string{"network.wg0.private_key": "private-key-value"}
	if !ValidSecretOptions(config, secrets) {
		t.Fatal("valid secret option was rejected")
	}
	if ValidSecretOptions(config, map[string]string{"network.wg0.public_key": "value"}) {
		t.Fatal("invalid secret option was accepted")
	}
	if ValidSecretOptions(Configuration{}, secrets) {
		t.Fatal("secret for an unmanaged section was accepted")
	}
	lua := configurationLua(config, true, nil, secrets)
	if !strings.Contains(lua, "uci:set('network','wg0','private_key','private-key-value')") {
		t.Fatal("secret was not written by the transaction")
	}
	stable := configurationStableReadScript(config, nil, secrets)
	if !strings.Contains(stable, "preshared_key") && !strings.Contains(stable, "private_key-value") && !strings.Contains(stable, "secrets") {
		t.Fatal("stable read did not include the secret check")
	}
	confirm := configurationConfirmScript("token", config, nil, secrets)
	if !strings.Contains(confirm, "network.wg0.private_key") {
		t.Fatal("confirmation did not verify the secret")
	}
}

func TestSafetyStateRedactsNetworkSecrets(t *testing.T) {
	for _, fragment := range []string{"private_key", "preshared_key", "auth_user_pass"} {
		if !strings.Contains(safetyStateLua, fragment) {
			t.Fatalf("safety state does not redact %s", fragment)
		}
	}
}
