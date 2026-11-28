package router

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

type rapidTB struct{ *rapid.T }

func (rapidTB) Helper() {}

func (r rapidTB) TempDir() string {
	dir, err := os.MkdirTemp("", "glinet-restore-")
	if err != nil {
		r.Fatal(err)
	}
	r.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// TestRestoreStateMachineModel drives the real restore script through failed
// copies, failed service activation, retries, and repeated completion. The
// model is the transaction state machine:
//
//	prepared --restore--> restoring --success--> restored --repeat--> restored
//	                           `--failure--> restoring (retry)
//
// The invariant is that a transaction reports restored only after every
// package and service has been restored.
func TestRestoreStateMachineModel(t *testing.T) {
	// The fixture executes real shell scripts, so keep the session short while
	// covering every failure category.
	if flag.Lookup("rapid.checks") != nil {
		if err := flag.Set("rapid.checks", "30"); err != nil {
			t.Fatal(err)
		}
	}
	packages := []string{"network", "firewall", "samba4"}
	rapid.Check(t, func(t *rapid.T) {
		tb := rapidTB{t}
		f := newConfigurationWorld(tb)
		f.delay = "1"
		root := f.base.root
		_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
		failing := map[string]bool{}
		for _, name := range packages {
			if rapid.Bool().Draw(t, "fail-"+name) {
				failing[name] = true
			}
		}
		failService := rapid.Bool().Draw(t, "fail-service")
		helper := "#!/bin/sh\nfor target; do :; done\ncase \"$(basename \"$target\")\" in\n"
		for name := range failing {
			helper += name + ") exit 1 ;;\n"
		}
		helper += "esac\nexec /bin/cp \"$@\"\n"
		if err := os.WriteFile(filepath.Join(root, "bin", "cp"), []byte(helper), 0700); err != nil {
			t.Fatal(err)
		}
		service := "#!/bin/sh\nif [ -f " + quote(filepath.Join(root, "fail-service")) + " ]; then exit 1; fi\nexit 0\n"
		if err := os.WriteFile(filepath.Join(root, "service"), []byte(service), 0700); err != nil {
			t.Fatal(err)
		}
		if failService {
			_ = os.WriteFile(filepath.Join(root, "fail-service"), nil, 0600)
		}
		dir := filepath.Join(root, "transaction")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		writeFile(tb, filepath.Join(dir, "token"), "token")
		for _, name := range packages {
			writeFile(tb, filepath.Join(dir, name), "original-"+name)
		}
		restore := strings.Replace(configurationRestoreScript("token", reloadPlan{DNS: true, DHCP: true, Firewall: true}), "for package in "+configurationPackages+"; do", "for package in "+strings.Join(packages, " ")+"; do", 1)
		result := func() string {
			data, err := os.ReadFile(filepath.Join(root, "results", "token"))
			if err != nil {
				return ""
			}
			return string(data)
		}
		if out, err := f.run(restore); err != nil {
			t.Logf("restore attempt failed: %s %v", out, err)
		}
		injected := len(failing) > 0 || failService
		switch result() {
		case recordRestored:
			if injected {
				t.Fatalf("restore reported success after failed copies (%v) or service activation (%v)", failing, failService)
			}
			for _, name := range packages {
				data, _ := os.ReadFile(filepath.Join(root, name))
				if string(data) != "original-"+name {
					t.Fatalf("restore reported success but %s is %q", name, data)
				}
			}
		case recordRestoring:
			if !injected {
				t.Fatalf("restore without injected failures did not record success")
			}
			if _, err := os.Stat(filepath.Join(dir, "firewall")); err != nil {
				t.Fatal("failed restore discarded the snapshot")
			}
			if err := os.Remove(filepath.Join(root, "fail-service")); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "bin", "cp"), []byte("#!/bin/sh\nexec /bin/cp \"$@\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if out, err := f.run(restore); err != nil {
				t.Fatalf("restore did not converge after transient failures: %s %v", out, err)
			}
			if got := result(); got != recordRestored {
				t.Fatalf("retry converged to %q, want %q", got, recordRestored)
			}
			for _, name := range packages {
				data, _ := os.ReadFile(filepath.Join(root, name))
				if string(data) != "original-"+name {
					t.Fatalf("retry left %s as %q", name, data)
				}
			}
			if out, err := f.run(restore); err != nil {
				t.Fatalf("repeated restore failed: %s %v", out, err)
			}
			if got := result(); got != recordRestored {
				t.Fatalf("repeated restore changed the outcome to %q", got)
			}
		case recordRestoreFailed:
			t.Fatal("a single restore attempt recorded an exhausted rollback")
		default:
			t.Fatalf("restore recorded unexpected outcome %q", result())
		}
	})
}
