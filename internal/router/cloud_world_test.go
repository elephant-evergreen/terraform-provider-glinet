package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newCloudConfigurationWorld(t *testing.T) *configurationWorld {
	t.Helper()
	f := newConfigurationWorld(t)
	root := f.base.root
	ubus := `#!/bin/sh
case "$*" in
  *get_config*) printf '{"result":{"cloud_enable":%s}}\n' "$(cat ` + root + `/cloud-enabled)";;
  *set_config*)
    case "$*" in
      *cloud_enable*true*)
        printf true > ` + root + `/cloud-enabled
        printf 1 > ` + root + `/gl-cloud-running;;
      *)
        printf false > ` + root + `/cloud-enabled
        printf 0 > ` + root + `/gl-cloud-running
        printf 0 > ` + root + `/rtty-running
        rm -f ` + root + `/gl-cloud-registered ` + root + `/rtty-registered ` + root + `/mpifd-registered;;
    esac
    printf '{}\n';;
  *service*add*)
    name=$(printf '%s' "$*" | sed -n 's/.*"name":"\([^"]*\)".*/\1/p')
    if [ -n "$name" ]; then printf 1 > ` + root + `/"$name"-registered; fi
    printf '{}\n';;
  *service*delete*)
    name=$(printf '%s' "$*" | sed -n 's/.*"name":"\([^"]*\)".*/\1/p')
    if [ -n "$name" ]; then rm -f ` + root + `/"$name"-registered; fi
    printf '{}\n';;
  *service*list*)
    name=$(printf '%s' "$*" | sed -n 's/.*"name":"\([^"]*\)".*/\1/p')
    registered_file=` + root + `/"$name"-registered
    if [ -n "$name" ] && [ -f "$registered_file" ] && [ "$(cat "$registered_file")" = 1 ]; then
      printf '{"%s":{}}\n' "$name"
    else
      printf '{}\n'
    fi;;
  *) printf '{}\n';;
esac
`
	if err := os.WriteFile(filepath.Join(root, "bin", "ubus"), []byte(ubus), 0700); err != nil {
		t.Fatal(err)
	}
	jsonfilter := "#!/bin/sh\ncat " + filepath.Join(root, "cloud-enabled") + "\n"
	if err := os.WriteFile(filepath.Join(root, "bin", "jsonfilter"), []byte(jsonfilter), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cloud-enabled"), []byte("false"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gl-cloud", "rtty", "mptun", "mpifd"} {
		if err := os.WriteFile(filepath.Join(root, name+"-running"), []byte("0"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for name, value := range map[string]string{"gl-cloud": "1", "rtty": "0", "mptun": "1", "mpifd": "0"} {
		if err := os.WriteFile(filepath.Join(root, name+"-registered"), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "gl-cloud"), []byte("original-gl-cloud"), 0600); err != nil {
		t.Fatal(err)
	}
	pidof := `#!/bin/sh
name=$1
state=` + root + `/$name-running
if [ -f "$state" ] && [ "$(cat "$state")" = 1 ]; then
  printf 123
  exit 0
fi
exit 1
`
	if err := os.WriteFile(filepath.Join(root, "bin", "pidof"), []byte(pidof), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gl-cloud", "rtty", "mptun", "mpifd"} {
		body := "#!/bin/sh\nprintf '%s %s\\n' \"$0\" \"$1\" >> " + filepath.Join(root, "service-calls") + "\nexit 0\n"
		if err := os.WriteFile(filepath.Join(root, "service-"+name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func TestCloudConfigurationWorld(t *testing.T) {
	f := newCloudConfigurationWorld(t)
	root := f.base.root
	if err := os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600); err != nil {
		t.Fatal(err)
	}
	spec := Configuration{{Package: "gl-cloud", Name: "@cloud[0]", Type: "cloud", Options: map[string]string{ManagedOption: "1"}}}
	plan := reloadPlan{Cloud: true, CloudEnabled: false, ServiceSnapshot: true}
	if output, err := f.run(configurationApplyScript(spec, "token", plan, nil)); err != nil {
		t.Fatalf("Apply failed: %s %v", output, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "transaction", "ready")); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if output, err := f.run(configurationConfirmScriptWithPlan("token", spec, plan)); err != nil {
		t.Fatalf("Confirm failed: %s %v", output, err)
	}
	enabled, _ := os.ReadFile(filepath.Join(root, "cloud-enabled"))
	if strings.TrimSpace(string(enabled)) != "false" {
		t.Fatalf("Cloud was not disabled: %q", enabled)
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "lock")); os.IsNotExist(err) {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(root, "transaction")); !os.IsNotExist(err) {
		t.Fatal("Transaction was not cleaned up")
	}
}

func TestCloudConfigurationRollbackRestoresServices(t *testing.T) {
	f := newCloudConfigurationWorld(t)
	root := f.base.root
	if err := os.WriteFile(filepath.Join(root, "boot-id"), []byte("initial\n"), 0600); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(root, "gl-cloud"))
	if err != nil {
		t.Fatal(err)
	}
	spec := Configuration{{Package: "gl-cloud", Name: "@cloud[0]", Type: "cloud", Options: map[string]string{ManagedOption: "1"}}}
	plan := reloadPlan{Cloud: true, CloudEnabled: false, ServiceSnapshot: true}
	if output, err := f.run(configurationApplyScript(spec, "token", plan, nil)); err != nil {
		t.Fatalf("Apply failed: %s %v", output, err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "lock")); os.IsNotExist(err) {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	restored, _ := os.ReadFile(filepath.Join(root, "gl-cloud"))
	if string(restored) != string(original) {
		t.Fatalf("GoodCloud package was not restored: %q", restored)
	}
	calls, _ := os.ReadFile(filepath.Join(root, "service-calls"))
	for _, service := range []string{"service-rtty", "service-mpifd"} {
		if !strings.Contains(string(calls), service) {
			t.Fatalf("Rollback did not restore %s: %s", service, calls)
		}
	}
	for _, name := range []string{"gl-cloud", "mptun"} {
		registered, _ := os.ReadFile(filepath.Join(root, name+"-registered"))
		if strings.TrimSpace(string(registered)) != "1" {
			t.Fatalf("Rollback did not restore the %s registration", name)
		}
	}
}

func TestCloudRestoreStoppedRegisteredService(t *testing.T) {
	f := newCloudConfigurationWorld(t)
	root := f.base.root
	if err := os.Remove(filepath.Join(root, "gl-cloud-registered")); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	before := "gl_cloud_running=0\ngl_cloud_registered=1\nrtty_running=0\nrtty_registered=0\nmptun_running=0\nmptun_registered=0\nmpifd_running=0\nmpifd_registered=0\n"
	if err := os.WriteFile(filepath.Join(dir, "service-before"), []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	script := "set -eu\ncd " + quote(dir) + "\n" + cloudRestoreScript()
	if output, err := f.run(script); err != nil {
		result, _ := os.ReadFile(filepath.Join(dir, "service-restore"))
		calls, _ := os.ReadFile(filepath.Join(root, "service-calls"))
		t.Fatalf("Restore failed: %s %v result=%q calls=%q", output, err, result, calls)
	}
	registered, _ := os.ReadFile(filepath.Join(root, "gl-cloud-registered"))
	if strings.TrimSpace(string(registered)) != "1" {
		t.Fatal("Stopped, registered gl-cloud service was not restored")
	}
}

func TestCloudVerifyFailsWhenProcessRemains(t *testing.T) {
	for _, name := range []string{"gl-cloud", "rtty"} {
		t.Run(name, func(t *testing.T) {
			f := newCloudConfigurationWorld(t)
			root := f.base.root
			if err := os.WriteFile(filepath.Join(root, name+"-running"), []byte("1"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := f.run("set -eu\n" + cloudVerifyScript(false) + "exit 0\n"); err == nil {
				t.Fatalf("Disabled GoodCloud accepted a running %s process", name)
			}
		})
	}
}

func TestCloudRestoreFailureKeepsResult(t *testing.T) {
	f := newCloudConfigurationWorld(t)
	root := f.base.root
	for _, name := range []string{"gl-cloud", "rtty", "mptun", "mpifd"} {
		body := "#!/bin/sh\nexit 1\n"
		if err := os.WriteFile(filepath.Join(root, "service-"+name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	before := "gl_cloud_running=1\ngl_cloud_registered=0\nrtty_running=1\nrtty_registered=0\nmptun_running=1\nmptun_registered=0\nmpifd_running=1\nmpifd_registered=0\n"
	if err := os.WriteFile(filepath.Join(dir, "service-before"), []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	script := "set -eu\ncd " + quote(dir) + "\n" + cloudRestoreScript()
	if _, err := f.run(script); err == nil {
		t.Fatal("Failed service restoration was reported as success")
	}
	result, err := os.ReadFile(filepath.Join(dir, "service-restore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != "1" {
		t.Fatalf("Service restore result = %q, want 1", result)
	}
}
