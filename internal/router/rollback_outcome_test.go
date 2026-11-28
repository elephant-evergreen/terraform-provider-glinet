package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTransactionOutcome(t *testing.T) {
	for input, want := range map[string]transactionOutcome{
		"rolled_back\n": outcomeRolledBack,
		"committed":     outcomeCommitted,
		"cancelled":     outcomeCancelled,
		"superseded":    outcomeSuperseded,
		"missing":       outcomeUnknown,
		"timeout":       outcomeUnknown,
		"":              outcomeUnknown,
	} {
		if got := parseTransactionOutcome([]byte(input)); got != want {
			t.Errorf("parseTransactionOutcome(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestRollbackScriptReportsOutcome(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
		want  string
	}{
		{"committed result", func(t *testing.T, root string) {
			writeOutcome(t, root, "confirm")
		}, "committed"},
		{"rolled back result", func(t *testing.T, root string) {
			writeOutcome(t, root, "rollback")
		}, "rolled_back"},
		{"cancelled result", func(t *testing.T, root string) {
			writeOutcome(t, root, "cancel")
		}, "cancelled"},
		{"missing transaction", func(t *testing.T, root string) {}, "missing"},
		{"superseded transaction", func(t *testing.T, root string) {
			writeTransaction(t, root, "other")
		}, "superseded"},
		{"committed decision", func(t *testing.T, root string) {
			writeTransaction(t, root, "token")
			writeFile(t, filepath.Join(root, "transaction", "decision"), "confirm")
		}, "committed"},
		{"rollback request", func(t *testing.T, root string) {
			writeTransaction(t, root, "token")
			go func() {
				time.Sleep(250 * time.Millisecond)
				_ = os.RemoveAll(filepath.Join(root, "transaction"))
				writeOutcome(t, root, "rollback")
			}()
		}, "rolled_back"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := newConfigurationWorld(t)
			root := f.base.root
			test.setup(t, root)
			script := configurationRollbackScript("token")
			script = replaceSleep(script, "sleep 10", "sleep 0.05")
			output, err := f.run(script)
			if err != nil {
				t.Fatalf("Rollback script failed: %s %v", output, err)
			}
			if got := string(output); got != test.want {
				t.Fatalf("Rollback outcome = %q, want %q", got, test.want)
			}
		})
	}
}

func writeTransaction(t *testing.T, root, token string) {
	t.Helper()
	dir := filepath.Join(root, "transaction")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "token"), token)
}

func writeOutcome(t *testing.T, root, outcome string) {
	t.Helper()
	dir := filepath.Join(root, "results")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "token"), outcome)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func replaceSleep(script, old, new string) string {
	for {
		next := strings.Replace(script, old, new, 1)
		if next == script {
			return script
		}
		script = next
	}
}
