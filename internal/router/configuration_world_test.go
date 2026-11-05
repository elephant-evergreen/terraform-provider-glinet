package router

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type configurationWorld struct {
	base  *fixtureRunner
	delay string
}

func newConfigurationWorld(t *testing.T) *configurationWorld {
	t.Helper()
	f := newFixture(t)
	for _, p := range strings.Fields(configurationPackages) {
		if err := os.WriteFile(filepath.Join(f.root, p), []byte("original-"+p), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"lua", "fw3", "ubus", "jsonfilter", "pidof"} {
		body := "#!/bin/sh\ncat >/dev/null\n"
		if name == "fw3" || name == "pidof" {
			body = "#!/bin/sh\nexit 0\n"
		}
		if name == "ubus" || name == "jsonfilter" {
			body = "#!/bin/sh\nprintf '{}\\n'\n"
		}
		if err := os.WriteFile(filepath.Join(f.root, "bin", name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.root, "bin", "uci"), []byte("#!/bin/sh\ncase \"$*\" in changes) exit 0 ;; *adguardhome*) exit 1 ;; *) exit 0 ;; esac\n"), 0700); err != nil {
		t.Fatal(err)
	}
	service := filepath.Join(f.root, "service")
	if err := os.WriteFile(service, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, name := range []string{"pid", "reload.pid"} {
			data, _ := os.ReadFile(filepath.Join(f.root, "transaction", name))
			pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if pid > 1 {
				_ = syscall.Kill(pid, syscall.SIGTERM)
			}
		}
	})
	return &configurationWorld{base: f, delay: "3"}
}
func (f *configurationWorld) run(script string) ([]byte, error) {
	root := f.base.root
	script = strings.ReplaceAll(script, preflight, "set -eu\n")
	script = strings.ReplaceAll(script, "sleep 300 &", "sleep "+f.delay+" &")
	script = strings.ReplaceAll(script, "sleep 2\n", "sleep 0.1\n")
	script = strings.ReplaceAll(script, "/proc/sys/kernel/random/boot_id", filepath.Join(root, "boot-id"))
	script = strings.ReplaceAll(script, transactionDir, filepath.Join(root, "transaction"))
	script = strings.ReplaceAll(script, configurationLock, filepath.Join(root, "lock"))
	script = strings.ReplaceAll(script, recoveryService, filepath.Join(root, "recovery"))
	script = strings.ReplaceAll(script, "/etc/config/", root+"/")
	script = strings.ReplaceAll(script, "/etc/init.d/dnsmasq", filepath.Join(root, "service"))
	script = strings.ReplaceAll(script, "/etc/init.d/firewall", filepath.Join(root, "service"))
	script = strings.ReplaceAll(script, filepath.Join(root, "recovery")+" enable", "true")
	script = strings.ReplaceAll(script, filepath.Join(root, "recovery")+" disable", "true")
	binary, _ := os.Executable()
	cmd := exec.Command("/bin/sh", "-s")
	cmd.Stdin = strings.NewReader(script)
	cmd.Env = []string{"PATH=" + filepath.Join(root, "bin") + ":/usr/bin:/bin", "GLINET_TEST_BINARY=" + binary, "GLINET_TEST_ROOT=" + root, "GORACE=atexit_sleep_ms=0"}
	return cmd.CombinedOutput()
}
func TestConfigurationWorldConfirmationAndRecovery(t *testing.T) {
	for _, confirm := range []bool{true, false} {
		t.Run(map[bool]string{true: "confirm", false: "recover"}[confirm], func(t *testing.T) {
			f := newConfigurationWorld(t)
			root := f.base.root
			_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
			spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
			script := configurationApplyScript(spec, nil, "token", false)
			script = strings.Replace(script, configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
			if output, err := f.run(script); err != nil {
				t.Fatalf("Apply failed: %s %v", output, err)
			}
			changed, _ := os.ReadFile(filepath.Join(root, "firewall"))
			if string(changed) != "changed" {
				t.Fatal("Fixture did not mutate its configuration")
			}
			if confirm {
				deadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					if _, err := os.Stat(filepath.Join(root, "transaction", "ready")); err == nil {
						break
					}
					time.Sleep(20 * time.Millisecond)
				}
				if output, err := f.run(configurationConfirmScript("token", spec, nil)); err != nil {
					t.Fatalf("Confirm failed: %s %v", output, err)
				}
			}
			deadline := time.Now().Add(7 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Stat(filepath.Join(root, "lock")); os.IsNotExist(err) {
					break
				}
				time.Sleep(30 * time.Millisecond)
			}
			actual, _ := os.ReadFile(filepath.Join(root, "firewall"))
			want := "original-firewall"
			if confirm {
				want = "changed"
			}
			if string(actual) != want {
				t.Fatalf("Unexpected world after transaction: %q", actual)
			}
			for _, name := range []string{"lock", "transaction", "recovery"} {
				if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
					t.Fatalf("Transaction left %s", name)
				}
			}
		})
	}
}

func TestConfigurationWorldFailedWatchdogNeverMutates(t *testing.T) {
	f := newConfigurationWorld(t)
	root := f.base.root
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
	_ = os.WriteFile(filepath.Join(root, "fail-watchdog"), nil, 0600)
	spec := configurationFixture()
	script := strings.Replace(configurationApplyScript(spec, nil, "token", false), configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
	if _, err := f.run(script); err == nil {
		t.Fatal("Failed watchdog was accepted")
	}
	actual, _ := os.ReadFile(filepath.Join(root, "firewall"))
	if string(actual) != "original-firewall" {
		t.Fatal("Failed watchdog changed configuration")
	}
	for _, name := range []string{"lock", "transaction", "recovery"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("Failed preparation left %s", name)
		}
	}
}

func TestConfigurationWorldRebootRearmsAndCanConfirm(t *testing.T) {
	f := newConfigurationWorld(t)
	f.delay = "8"
	root := f.base.root
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
	spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
	script := strings.Replace(configurationApplyScript(spec, nil, "token", false), configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
	if output, err := f.run(script); err != nil {
		t.Fatalf("Apply failed: %s %v", output, err)
	}
	pid, _ := os.ReadFile(filepath.Join(root, "transaction", "pid"))
	value, _ := strconv.Atoi(strings.TrimSpace(string(pid)))
	if value < 2 {
		t.Fatal("Watchdog did not start")
	}
	_ = syscall.Kill(value, syscall.SIGTERM)
	_ = os.Remove(filepath.Join(root, "lock"))
	_ = os.Remove(filepath.Join(root, "transaction", "ready"))
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("rebooted\n"), 0600)
	service, err := os.ReadFile(filepath.Join(root, "recovery"))
	if err != nil {
		t.Fatal(err)
	}
	if output, err := f.run(string(service) + "\nstart\n"); err != nil {
		t.Fatalf("Boot service failed: %s %v", output, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "transaction", "ready")); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if output, err := f.run(configurationConfirmScript("token", spec, nil)); err != nil {
		t.Fatalf("Rebooted transaction could not confirm: %s %v", output, err)
	}
	actual, _ := os.ReadFile(filepath.Join(root, "firewall"))
	if string(actual) != "changed" {
		t.Fatal("Confirmed reboot lost applied configuration")
	}
}

func TestConfigurationWorldRejectsAdoptionWhileRecoveryIsPending(t *testing.T) {
	f := newConfigurationWorld(t)
	f.delay = "8"
	root := f.base.root
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
	spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
	script := strings.Replace(configurationApplyScript(spec, nil, "token", false), configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
	if out, err := f.run(script); err != nil {
		t.Fatalf("Apply failed: %s %v", out, err)
	}
	if _, err := f.run(configurationStableReadScript(spec)); err == nil {
		t.Fatal("Pending configuration was accepted as stable")
	}
	if _, err := os.Stat(filepath.Join(root, "transaction", "firewall")); err != nil {
		t.Fatal("Rejected adoption lost recovery snapshot")
	}
}

func TestConfigurationWorldInterruptedConfirmationCanFinishOnRetry(t *testing.T) {
	f := newConfigurationWorld(t)
	f.delay = "8"
	root := f.base.root
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
	spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
	script := strings.Replace(configurationApplyScript(spec, nil, "token", false), configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
	if out, err := f.run(script); err != nil {
		t.Fatalf("Apply failed: %s %v", out, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "transaction", "ready")); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	confirm := strings.Replace(configurationConfirmScript("token", spec, nil), configurationCleanup(), "exit 0\n", 1)
	if out, err := f.run(confirm); err != nil {
		t.Fatalf("Confirmation decision failed: %s %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(root, "transaction", "decision")); err != nil {
		t.Fatal("Confirmation decision did not persist")
	}
	if out, err := f.run(configurationStableReadScript(spec)); err != nil {
		t.Fatalf("Retry did not finish confirmed cleanup: %s %v", out, err)
	}
	for _, name := range []string{"lock", "transaction", "recovery"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("Confirmed retry left %s", name)
		}
	}
	actual, _ := os.ReadFile(filepath.Join(root, "firewall"))
	if string(actual) != "changed" {
		t.Fatal("Confirmed cleanup restored old configuration")
	}
}

func TestConfigurationWorldInterruptedRestoreResumesAfterReboot(t *testing.T) {
	f := newConfigurationWorld(t)
	f.delay = "2"
	root := f.base.root
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
	helper := "#!/bin/sh\nfor target; do :; done\nif [ -f \"$GLINET_TEST_ROOT/fail-copy\" ] && [ \"$target\" = \"$GLINET_TEST_ROOT/firewall\" ]; then exit 1; fi\nexec /bin/cp \"$@\"\n"
	_ = os.WriteFile(filepath.Join(root, "bin", "cp"), []byte(helper), 0700)
	spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
	script := strings.Replace(configurationApplyScript(spec, nil, "token", false), configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
	if out, err := f.run(script); err != nil {
		t.Fatalf("Apply failed: %s %v", out, err)
	}
	_ = os.WriteFile(filepath.Join(root, "fail-copy"), nil, 0600)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "transaction", "decision")); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(root, "transaction", "firewall")); err != nil {
		t.Fatal("Failed restore discarded its snapshot")
	}
	pid, _ := os.ReadFile(filepath.Join(root, "transaction", "pid"))
	value, _ := strconv.Atoi(strings.TrimSpace(string(pid)))
	if value < 2 {
		t.Fatal("Watchdog did not start")
	}
	_ = syscall.Kill(value, syscall.SIGTERM)
	_ = os.Remove(filepath.Join(root, "fail-copy"))
	_ = os.Remove(filepath.Join(root, "lock"))
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("rebooted\n"), 0600)
	service, err := os.ReadFile(filepath.Join(root, "recovery"))
	if err != nil {
		t.Fatal(err)
	}
	if out, err := f.run(string(service) + "\nstart\n"); err != nil {
		t.Fatalf("Recovery restart failed: %s %v", out, err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "lock")); os.IsNotExist(err) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, p := range strings.Fields(configurationPackages) {
		actual, _ := os.ReadFile(filepath.Join(root, p))
		if string(actual) != "original-"+p {
			t.Fatalf("Restart did not restore %s", p)
		}
	}
	for _, name := range []string{"lock", "transaction", "recovery"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("Completed recovery left %s", name)
		}
	}
}
