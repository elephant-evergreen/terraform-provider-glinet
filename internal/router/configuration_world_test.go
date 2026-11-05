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
				_ = syscall.Kill(-pid, syscall.SIGTERM)
			}
		}
	})
	return &configurationWorld{base: f, delay: "3"}
}
func (f *configurationWorld) run(script string) ([]byte, error) {
	root := f.base.root
	script = strings.ReplaceAll(script, preflight, "set -eu\n")
	script = strings.ReplaceAll(script, "remaining=300", "remaining="+f.delay)
	script = strings.ReplaceAll(script, "/proc/sys/kernel/random/boot_id", filepath.Join(root, "boot-id"))
	script = strings.ReplaceAll(script, resultDir, filepath.Join(root, "results"))
	script = strings.ReplaceAll(script, transactionDir, filepath.Join(root, "transaction"))
	script = strings.ReplaceAll(script, configurationLock, filepath.Join(root, "lock"))
	script = strings.ReplaceAll(script, recoveryService, filepath.Join(root, "recovery"))
	script = strings.ReplaceAll(script, "/etc/config/", root+"/")
	script = strings.ReplaceAll(script, "/etc/rc.d/", root+"/rc/")
	script = strings.ReplaceAll(script, "/etc/init.d/network", filepath.Join(root, "service"))
	script = strings.ReplaceAll(script, "/etc/init.d/gl-vlan-port-map", filepath.Join(root, "service"))
	script = strings.ReplaceAll(script, "/etc/init.d/dnsmasq", filepath.Join(root, "service"))
	script = strings.ReplaceAll(script, "/etc/init.d/firewall", filepath.Join(root, "service"))
	script = strings.ReplaceAll(script, filepath.Join(root, "recovery")+" enable", "true")
	script = strings.ReplaceAll(script, filepath.Join(root, "recovery")+" disable", "true")
	binary, _ := os.Executable()
	cmd := exec.Command("/bin/sh", "-s")
	cmd.Stdin = strings.NewReader(script)
	cmd.Env = []string{"PATH=" + filepath.Join(root, "bin") + ":/usr/bin:/bin", "GLINET_TEST_BINARY=" + binary, "GLINET_TEST_ROOT=" + root, "GORACE=atexit_sleep_ms=0"}
	output, err := cmd.CombinedOutput()
	if err == nil && strings.Contains(script, "touch activate") {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			data, _ := os.ReadFile(filepath.Join(root, "firewall"))
			if string(data) == "changed" {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	return output, err
}
func TestConfigurationWorldConfirmationAndRecovery(t *testing.T) {
	for _, confirm := range []bool{true, false} {
		t.Run(map[bool]string{true: "confirm", false: "recover"}[confirm], func(t *testing.T) {
			f := newConfigurationWorld(t)
			root := f.base.root
			_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
			spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
			script := configurationApplyScript(spec, nil, "token", reloadPlan{DNS: true, DHCP: true, Firewall: true})
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
	script := strings.Replace(configurationApplyScript(spec, nil, "token", reloadPlan{DNS: true, DHCP: true, Firewall: true}), configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
	if _, err := f.run(script); err == nil {
		t.Fatal("Failed watchdog was accepted")
	}
	actual, _ := os.ReadFile(filepath.Join(root, "firewall"))
	if string(actual) != "original-firewall" {
		t.Fatal("Failed watchdog changed configuration")
	}
	if _, err := os.Stat(filepath.Join(root, "transaction", "firewall")); err != nil {
		t.Fatal("Uncertain worker startup lost its recovery snapshot")
	}
}

func TestConfigurationWorldUnexpectedRebootRestoresBeforeConfirmation(t *testing.T) {
	f := newConfigurationWorld(t)
	f.delay = "8"
	root := f.base.root
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
	spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
	script := strings.Replace(configurationApplyScript(spec, nil, "token", reloadPlan{DNS: true, DHCP: true, Firewall: true}), configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
	if output, err := f.run(script); err != nil {
		t.Fatalf("Apply failed: %s %v", output, err)
	}
	pid, _ := os.ReadFile(filepath.Join(root, "transaction", "pid"))
	value, _ := strconv.Atoi(strings.TrimSpace(string(pid)))
	if value < 2 {
		t.Fatal("Watchdog did not start")
	}
	_ = syscall.Kill(-value, syscall.SIGTERM)
	_ = os.RemoveAll(filepath.Join(root, "lock"))
	_ = os.RemoveAll(filepath.Join(root, "lock-actor-token"))
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("rebooted\n"), 0600)
	if _, err := f.run(configurationConfirmScript("token", spec, nil)); err == nil {
		t.Fatal("Stale readiness from an earlier boot was accepted")
	}
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
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "lock")); os.IsNotExist(err) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	actual, _ := os.ReadFile(filepath.Join(root, "firewall"))
	if string(actual) != "original-firewall" {
		t.Fatal("Unexpected reboot did not restore the original configuration")
	}

}

func TestConfigurationWorldRejectsAdoptionWhileRecoveryIsPending(t *testing.T) {
	f := newConfigurationWorld(t)
	f.delay = "8"
	root := f.base.root
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
	spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
	script := strings.Replace(configurationApplyScript(spec, nil, "token", reloadPlan{DNS: true, DHCP: true, Firewall: true}), configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
	if out, err := f.run(script); err != nil {
		t.Fatalf("Apply failed: %s %v", out, err)
	}
	if _, err := f.run(configurationStableReadScript(spec, nil)); err == nil {
		t.Fatal("Pending configuration was accepted as stable")
	}
	if _, err := os.Stat(filepath.Join(root, "transaction", "firewall")); err != nil {
		t.Fatal("Rejected adoption lost recovery snapshot")
	}
}

func TestConfigurationWorldInterruptedConfirmationFinishesWithoutCallerCleanup(t *testing.T) {
	f := newConfigurationWorld(t)
	f.delay = "8"
	root := f.base.root
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
	spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
	script := strings.Replace(configurationApplyScript(spec, nil, "token", reloadPlan{DNS: true, DHCP: true, Firewall: true}), configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
	if out, err := f.run(script); err != nil {
		t.Fatalf("Apply failed: %s %v", out, err)
	}
	confirm := "ln " + quote(transactionDir+"/confirm") + " " + quote(transactionDir+"/decision") + "\n"
	if out, err := f.run(confirm); err != nil {
		t.Fatalf("Atomic confirmation failed: %s %v", out, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "lock")); os.IsNotExist(err) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if out, err := f.run(configurationStableReadScript(spec, nil)); err != nil {
		t.Fatalf("Owner did not finish confirmed cleanup: %s %v", out, err)
	}
	for _, name := range []string{"lock", "transaction", "recovery"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("Confirmed owner left %s", name)
		}
	}
	actual, _ := os.ReadFile(filepath.Join(root, "firewall"))
	if string(actual) != "changed" {
		t.Fatal("Confirmed owner restored old configuration")
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
	script := strings.Replace(configurationApplyScript(spec, nil, "token", reloadPlan{DNS: true, DHCP: true, Firewall: true}), configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
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
	_ = syscall.Kill(-value, syscall.SIGTERM)
	_ = os.Remove(filepath.Join(root, "fail-copy"))
	_ = os.RemoveAll(filepath.Join(root, "lock"))
	_ = os.RemoveAll(filepath.Join(root, "lock-actor-token"))
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

func TestConfigurationWorldFullRebootBranchConfirmsAndRecovers(t *testing.T) {
	for _, confirm := range []bool{true, false} {
		t.Run(map[bool]string{true: "confirm", false: "recover"}[confirm], func(t *testing.T) {
			f := newConfigurationWorld(t)
			f.delay = "2"
			root := f.base.root
			_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
			_ = os.WriteFile(filepath.Join(root, "bin", "reboot"), []byte("#!/bin/sh\nprintf reboot >> \"$GLINET_TEST_ROOT/reboots\"\n"), 0700)
			spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
			script := strings.Replace(configurationApplyScript(spec, nil, "token", reloadPlan{Reboot: true, DNS: true, DHCP: true, Firewall: true}), configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
			if out, err := f.run(script); err != nil {
				t.Fatalf("Apply failed: %s %v", out, err)
			}
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Stat(filepath.Join(root, "reboots")); err == nil {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			reboot := func(id string) {
				pid, _ := os.ReadFile(filepath.Join(root, "transaction", "pid"))
				value, _ := strconv.Atoi(strings.TrimSpace(string(pid)))
				if value > 1 {
					_ = syscall.Kill(-value, syscall.SIGTERM)
				}
				_ = os.RemoveAll(filepath.Join(root, "lock"))
				_ = os.RemoveAll(filepath.Join(root, "lock-actor-token"))
				_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte(id+"\n"), 0600)
				service, err := os.ReadFile(filepath.Join(root, "recovery"))
				if err != nil {
					t.Fatal(err)
				}
				if out, err := f.run(string(service) + "\nstart\n"); err != nil {
					t.Fatalf("Boot service failed: %s %v", out, err)
				}
			}
			reboot("first-reboot")
			deadline = time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Stat(filepath.Join(root, "transaction", "ready")); err == nil {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if confirm {
				if out, err := f.run(configurationConfirmScript("token", spec, nil)); err != nil {
					t.Fatalf("Confirmation failed: %s %v", out, err)
				}
			} else {
				deadline = time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) {
					if _, err := os.Stat(filepath.Join(root, "transaction", "restored_boot")); err == nil {
						break
					}
					time.Sleep(20 * time.Millisecond)
				}
				reboot("restored-reboot")
			}
			deadline = time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Stat(filepath.Join(root, "lock")); os.IsNotExist(err) {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			actual, _ := os.ReadFile(filepath.Join(root, "firewall"))
			want := "original-firewall"
			if confirm {
				want = "changed"
			}
			if string(actual) != want {
				t.Fatalf("Unexpected reboot result: %q", actual)
			}
			for _, name := range []string{"lock", "transaction", "recovery"} {
				if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
					t.Fatalf("Reboot cleanup left %s", name)
				}
			}
		})
	}
}

func TestConfigurationWorldWaitsForReloadBeforeRestoring(t *testing.T) {
	f := newConfigurationWorld(t)
	f.delay = "2"
	root := f.base.root
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
	service := "#!/bin/sh\nif [ ! -f \"$GLINET_TEST_ROOT/service-started\" ]; then touch \"$GLINET_TEST_ROOT/service-started\"; sleep 4; touch \"$GLINET_TEST_ROOT/service-finished\"; fi\n"
	_ = os.WriteFile(filepath.Join(root, "service"), []byte(service), 0700)
	spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
	script := strings.Replace(configurationApplyScript(spec, nil, "token", reloadPlan{DNS: true, DHCP: true, Firewall: true}), configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
	if out, err := f.run(script); err != nil {
		t.Fatalf("Apply failed: %s %v", out, err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "transaction", "decision")); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(root, "service-finished")); os.IsNotExist(err) {
		actual, _ := os.ReadFile(filepath.Join(root, "firewall"))
		if string(actual) != "changed" {
			t.Fatal("Recovery raced the unfinished reload worker")
		}
	}
	deadline = time.Now().Add(7 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "lock")); os.IsNotExist(err) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	actual, _ := os.ReadFile(filepath.Join(root, "firewall"))
	if string(actual) != "original-firewall" {
		t.Fatal("Recovery did not run after reload quiesced")
	}
}

func TestConfigurationWorldCancellationBeforeActivationNeverStartsMutation(t *testing.T) {
	f := newConfigurationWorld(t)
	f.delay = "2"
	root := f.base.root
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
	_ = os.WriteFile(filepath.Join(root, "service"), []byte("#!/bin/sh\ntouch \"$GLINET_TEST_ROOT/service-called\"\n"), 0700)
	_ = os.WriteFile(filepath.Join(root, "bin", "reboot"), []byte("#!/bin/sh\ntouch \"$GLINET_TEST_ROOT/reboot-called\"\n"), 0700)
	helper := "#!/bin/sh\nfor target; do :; done\nif [ \"$target\" = \"$GLINET_TEST_ROOT/firewall\" ]; then touch \"$GLINET_TEST_ROOT/restore-called\"; fi\nexec /bin/cp \"$@\"\n"
	_ = os.WriteFile(filepath.Join(root, "bin", "cp"), []byte(helper), 0700)
	spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
	mutation := "touch " + quote(root+"/request-executed") + "\nprintf changed > " + quote(root+"/firewall") + "\n"
	script := strings.Replace(configurationApplyScript(spec, nil, "token", reloadPlan{Reboot: true, DNS: true, DHCP: true, Firewall: true}), configurationLua(spec, true, nil), mutation, 1)
	script = strings.Replace(script, "touch activate", "exit 0", 1)
	if out, err := f.run(script); err != nil {
		t.Fatalf("Preparation failed: %s %v", out, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "lock")); os.IsNotExist(err) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(root, "request-executed")); !os.IsNotExist(err) {
		t.Fatal("Unactivated worker mutated configuration")
	}
	for _, name := range []string{"service-called", "reboot-called", "restore-called"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("Canceled preparation caused %s", name)
		}
	}
	actual, _ := os.ReadFile(filepath.Join(root, "firewall"))
	if string(actual) != "original-firewall" {
		t.Fatal("Canceled preparation changed configuration")
	}
}

func TestConfigurationWorldRetiredCleanupResumesAfterInterruption(t *testing.T) {
	for _, stage := range []string{"rename", "retired-deletion"} {
		t.Run(stage, func(t *testing.T) {
			f := newConfigurationWorld(t)
			root := f.base.root
			_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
			spec := configurationFixture()
			prepare := strings.Replace(configurationApplyScript(spec, nil, "token", reloadPlan{}), "touch activate", "exit 0", 1)
			f.delay = "30"
			if out, err := f.run(prepare); err != nil {
				t.Fatalf("Preparation failed: %s %v", out, err)
			}
			pid, _ := os.ReadFile(filepath.Join(root, "transaction", "pid"))
			value, _ := strconv.Atoi(strings.TrimSpace(string(pid)))
			if value < 2 {
				t.Fatal("Worker did not start")
			}
			_ = syscall.Kill(-value, syscall.SIGTERM)
			service, err := os.ReadFile(filepath.Join(root, "recovery"))
			if err != nil {
				t.Fatal(err)
			}
			cleanup := configurationCleanup("token", "confirm")
			if stage == "rename" {
				cleanup = strings.Replace(cleanup, "sync\n", "exit 0\n", 1)
			} else {
				marker := "rm -f " + quote(recoveryService)
				cleanup = strings.Replace(cleanup, marker, "exit 0\n"+marker, 1)
			}
			if out, err := f.run(cleanup); err != nil {
				t.Fatalf("Interrupted cleanup failed: %s %v", out, err)
			}
			_ = os.RemoveAll(filepath.Join(root, "lock"))
			_ = os.RemoveAll(filepath.Join(root, "lock-actor-token"))
			_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("rebooted\n"), 0600)
			if out, err := f.run(string(service) + "\nstart\n"); err != nil {
				t.Fatalf("Boot cleanup failed: %s %v", out, err)
			}
			for range 2 {
				if out, err := f.run(configurationConfirmScript("token", spec, nil)); err != nil {
					t.Fatalf("Confirmed cleanup outcome could not be replayed after reboot: %s %v", out, err)
				}
			}

			for _, name := range []string{"transaction", "transaction-retired-token", "recovery", "lock", "lock-actor-token"} {
				if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
					t.Fatalf("Interrupted cleanup stranded %s", name)
				}
			}
		})
	}
}

func TestConfigurationWorldConfirmationWaitsForCompletedCleanup(t *testing.T) {
	f := newConfigurationWorld(t)
	root := f.base.root
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
	spec := Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"target": "REJECT"}}}
	releaseCleanup := filepath.Join(root, "release-cleanup")
	t.Cleanup(func() {
		_ = os.WriteFile(releaseCleanup, nil, 0600)
	})
	script := configurationApplyScript(spec, nil, "token", reloadPlan{Firewall: true})
	script = strings.Replace(script, configurationLua(spec, true, nil), "printf changed > "+quote(root+"/firewall")+"\n", 1)
	script = strings.ReplaceAll(script, "rm -rf "+quote(transactionDir+"-retired-token"), "while [ ! -f "+quote(releaseCleanup)+" ]; do sleep 0.05; done\nrm -rf "+quote(transactionDir+"-retired-token"))
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
	if _, err := os.Stat(filepath.Join(root, "transaction", "ready")); err != nil {
		t.Fatal("Apply never became ready")
	}
	confirmed := make(chan error, 1)
	go func() {
		_, err := f.run(configurationConfirmScript("token", spec, nil))
		confirmed <- err
	}()
	deadline = time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(filepath.Join(root, "results", "token"))
		if err == nil && string(data) == "confirm" {
			break
		}
		select {
		case err := <-confirmed:
			t.Fatalf("Confirmation returned before cleanup finished: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("Cleanup did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case err := <-confirmed:
		t.Fatalf("Confirmation returned while cleanup was still blocked: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if err := os.WriteFile(releaseCleanup, nil, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-confirmed:
		if err != nil {
			t.Fatalf("Confirmation failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Confirmation did not finish after cleanup was released")
	}
	if out, err := f.run(configurationConfirmScript("token", spec, nil)); err != nil {
		t.Fatalf("Lost SSH response could not be retried: %s %v", out, err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "results", "token")); err != nil || string(data) != "confirm" {
		t.Fatal("Confirmed outcome was not retained")
	}

	for _, name := range []string{"transaction", "transaction-retired-token", "recovery", "lock"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("Successful confirmation left %s", name)
		}
	}
}

func TestConfigurationWorldRolledBackKeyRotationNeverAcknowledgesSuccess(t *testing.T) {
	f := newConfigurationWorld(t)
	root := f.base.root
	_ = os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600)
	spec := Configuration{{Package: "wireless", Name: "wifi2g", Type: "wifi-iface", Options: map[string]string{"ssid": "Example"}}}
	keys := map[string]string{"wifi2g": "new-fixture-secret"}
	script := configurationApplyScript(spec, keys, "token", reloadPlan{Wireless: true})
	script = strings.Replace(script, configurationLua(spec, true, keys), "printf changed-key > "+quote(root+"/wireless")+"\nexit 1\n", 1)
	if out, err := f.run(script); err != nil {
		t.Fatalf("Preparation failed: %s %v", out, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "lock")); os.IsNotExist(err) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	actual, _ := os.ReadFile(filepath.Join(root, "wireless"))
	if string(actual) != "original-wireless" {
		t.Fatal("Failed rotation did not restore the original key fixture")
	}
	if out, err := f.run(configurationConfirmScript("token", spec, keys)); err == nil {
		t.Fatalf("Late confirmation accepted completed rollback: %s", out)
	}
	if data, err := os.ReadFile(filepath.Join(root, "results", "token")); err != nil || string(data) != "rollback" {
		t.Fatal("Recovery did not retain its rollback outcome")
	}
}
