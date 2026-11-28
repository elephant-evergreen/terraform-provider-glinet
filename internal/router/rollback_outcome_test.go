package router

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTransactionOutcome(t *testing.T) {
	for input, want := range map[string]transactionOutcome{
		"restored\n":       outcomeRolledBack,
		"restore_failed\n": outcomeRestoreFailed,
		"restoring\n":      outcomeRestoring,
		"committed":        outcomeCommitted,
		"cancelled":        outcomeCancelled,
		"superseded":       outcomeSuperseded,
		"missing":          outcomeUnknown,
		"timeout":          outcomeUnknown,
		"":                 outcomeUnknown,
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
		{"restored result", func(t *testing.T, root string) {
			writeOutcome(t, root, "restored")
		}, "restored"},
		{"restore failed result", func(t *testing.T, root string) {
			writeOutcome(t, root, "restore_failed")
		}, "restore_failed"},
		{"restore in progress result", func(t *testing.T, root string) {
			writeOutcome(t, root, "restoring")
		}, "missing"},
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
			writeFile(t, filepath.Join(root, "transaction", "recover"), "rollback")
		}, "pending"},
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
			if test.name == "rollback request" {
				decision, err := os.ReadFile(filepath.Join(root, "transaction", "decision"))
				if err != nil || string(decision) != "rollback" {
					t.Fatalf("Rollback request did not write the decision: %q %v", decision, err)
				}
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

func writeFile(t testTB, path, content string) {
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

type scriptedRunner struct {
	outputs [][]byte
	index   int
}

func (r *scriptedRunner) Run(context.Context, string) ([]byte, error) {
	if r.index >= len(r.outputs) {
		return []byte("pending"), nil
	}
	output := r.outputs[r.index]
	r.index++
	return output, nil
}

func TestRollbackTransactionPollsUntilOutcome(t *testing.T) {
	for name, want := range map[string]transactionOutcome{
		"restored":       outcomeRolledBack,
		"restore_failed": outcomeRestoreFailed,
		"confirm":        outcomeCommitted,
		"cancel":         outcomeCancelled,
		"missing":        outcomeUnknown,
	} {
		t.Run(name, func(t *testing.T) {
			runner := &scriptedRunner{outputs: [][]byte{[]byte("restoring"), []byte("pending"), []byte(name)}}
			client := NewClient(runner, nil)
			got, err := client.rollbackTransaction("token")
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("outcome = %v, want %v", got, want)
			}
		})
	}
}
