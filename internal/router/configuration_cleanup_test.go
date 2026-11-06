package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigurationCleanupIsIdempotent(t *testing.T) {
	f := newConfigurationWorld(t)
	root := f.base.root
	token := strings.Repeat("a", 32)
	for _, name := range []string{"lock", "transaction"} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "token"), []byte(token), 0600); err != nil {
			t.Fatal(err)
		}
	}
	script := configurationCleanup(token, "confirm")
	if output, err := f.run(script); err != nil {
		t.Fatalf("first cleanup failed: %v: %s", err, output)
	}
	if output, err := f.run(script); err != nil {
		t.Fatalf("repeated cleanup failed: %v: %s", err, output)
	}
}

func TestConfigurationRollbackRetryIsBounded(t *testing.T) {
	watchdog := configurationApplyScript(Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}, strings.Repeat("a", 32), reloadPlan{Firewall: true})
	if !strings.Contains(watchdog, "attempts=$((attempts+1))") || !strings.Contains(watchdog, "exit 75") {
		t.Fatal("watchdog rollback retry has no ceiling")
	}
	if strings.Contains(watchdog, "while ! /bin/sh restore; do sleep 5; done") {
		t.Fatal("watchdog rollback still retries without a bound")
	}
}
