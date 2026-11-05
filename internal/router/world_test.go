package router

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fixtureRunner struct {
	root  string
	delay string
}

func newFixture(t *testing.T) *fixtureRunner {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"uci", "wifi", "ubus", "jsonfilter", "start-stop-daemon"} {
		script := "#!/bin/sh\nexec \"$GLINET_TEST_BINARY\" -test.run=^TestRouterHelper$ -- " + name + " \"$@\"\n"
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := json.Marshal(map[string]string{"wifi2g": "wifi-iface", "wifi2g.mode": "ap", "wifi2g.ssid": "Before", "wifi2g.disabled": "0", "wifi2g.hidden": "0", "wifi2g.isolate": "1", "wifi2g.key": "fixture-secret-never-return"})
	if err := os.WriteFile(filepath.Join(root, "wireless"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "glversion"), []byte("4.11.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pid, err := os.ReadFile(filepath.Join(root, "lock", "pid"))
		if err == nil {
			value, _ := strconv.Atoi(strings.TrimSpace(string(pid)))
			if value > 1 {
				_ = syscall.Kill(value, syscall.SIGTERM)
			}
		}
	})
	return &fixtureRunner{root: root, delay: "10"}
}

func (f *fixtureRunner) Run(ctx context.Context, script string) ([]byte, error) {
	script = strings.ReplaceAll(script, "/tmp/terraform-provider-glinet-wifi", filepath.Join(f.root, "lock"))
	script = strings.ReplaceAll(script, "/etc/config/wireless", filepath.Join(f.root, "wireless"))
	script = strings.ReplaceAll(script, "/etc/glversion", filepath.Join(f.root, "glversion"))
	script = strings.ReplaceAll(script, "sleep 120 &", "sleep "+f.delay+" &")
	binary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-s")
	cmd.Env = []string{"PATH=" + filepath.Join(f.root, "bin") + ":/usr/bin:/bin", "GLINET_TEST_BINARY=" + binary, "GLINET_TEST_ROOT=" + f.root, "GORACE=atexit_sleep_ms=0"}
	cmd.Stdin = strings.NewReader(script)
	output, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil, &ExitError{Code: exit.ExitCode()}
	}
	return output, err
}

func TestRouterHelper(t *testing.T) {
	root := os.Getenv("GLINET_TEST_ROOT")
	if root == "" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		os.Exit(2)
	}
	name, args := args[1], args[2:]
	read := func(path string) map[string]string {
		data, err := os.ReadFile(path)
		if err != nil {
			os.Exit(2)
		}
		var values map[string]string
		if json.Unmarshal(data, &values) != nil {
			os.Exit(2)
		}
		return values
	}
	save := func(path string, values map[string]string) {
		data, _ := json.Marshal(values)
		if os.WriteFile(path, data, 0o600) != nil {
			os.Exit(2)
		}
	}
	switch name {
	case "uci":
		if len(args) > 0 && args[0] == "-q" {
			args = args[1:]
		}
		if len(args) < 2 {
			os.Exit(2)
		}
		file := filepath.Join(root, "wireless")
		stage := filepath.Join(root, "staged")
		switch args[0] {
		case "changes":
			if _, err := os.Stat(stage); err == nil {
				_, _ = os.Stdout.WriteString("wireless has staged changes\n")
			}
		case "get":
			if args[1] == "glconfig.general.mode" {
				_, _ = os.Stdout.WriteString("router\n")
				break
			}
			if _, err := os.Stat(stage); err == nil {
				file = stage
			}
			value, ok := read(file)[strings.TrimPrefix(args[1], "wireless.")]
			if !ok {
				os.Exit(1)
			}
			_, _ = os.Stdout.WriteString(value + "\n")
		case "set":
			if _, err := os.Stat(stage); err == nil {
				file = stage
			}
			values := read(file)
			key, value, ok := strings.Cut(strings.TrimPrefix(args[1], "wireless."), "=")
			if !ok {
				os.Exit(2)
			}
			values[key] = value
			save(stage, values)
		case "commit":
			if _, err := os.Stat(stage); err == nil {
				if os.Rename(stage, file) != nil {
					os.Exit(2)
				}
			}
		case "revert":
			_ = os.Remove(stage)
		default:
			os.Exit(2)
		}
	case "ubus":
		_, _ = os.Stdout.WriteString(`{"model":"GL.iNet GL-MT6000"}`)
	case "jsonfilter":
		data, _ := io.ReadAll(os.Stdin)
		var board struct {
			Model string `json:"model"`
		}
		if json.Unmarshal(data, &board) != nil {
			os.Exit(2)
		}
		_, _ = os.Stdout.WriteString(board.Model + "\n")
	case "wifi":
		if _, err := os.Stat(filepath.Join(root, "fail-reload")); err == nil {
			os.Exit(1)
		}
		file, err := os.OpenFile(filepath.Join(root, "reloads"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			os.Exit(2)
		}
		_, _ = file.WriteString("reload\n")
		_ = file.Close()
	case "start-stop-daemon":
		pidFile := ""
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "-p" {
				pidFile = args[i+1]
			}
		}
		if len(args) == 0 || pidFile == "" {
			os.Exit(2)
		}
		if args[0] == "-K" {
			pid, _ := os.ReadFile(pidFile)
			value, _ := strconv.Atoi(strings.TrimSpace(string(pid)))
			if value < 2 || syscall.Kill(value, syscall.SIGTERM) != nil {
				os.Exit(1)
			}
		} else {
			if _, err := os.Stat(filepath.Join(root, "fail-watchdog")); err == nil {
				os.Exit(1)
			}
			command := exec.Command("/bin/sh", args[len(args)-1])
			command.Env = os.Environ()
			command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			if command.Start() != nil {
				os.Exit(2)
			}
			if os.WriteFile(pidFile, []byte(strconv.Itoa(command.Process.Pid)), 0o600) != nil {
				_ = command.Process.Kill()
				os.Exit(2)
			}
			_ = command.Process.Release()
		}
	default:
		os.Exit(2)
	}
	os.Exit(0)
}

func TestSettingsChangeConfirmsAndPreservesSecrets(t *testing.T) {
	f := newFixture(t)
	client := NewClient(f)
	desired := Settings{Section: "wifi2g", SSID: `Example'$(echo unsafe)`, Hidden: true, Isolate: true}
	if err := client.Write(context.Background(), desired); err != nil {
		t.Fatal(err)
	}
	actual, err := client.Read(context.Background(), "wifi2g")
	if err != nil || actual != desired {
		t.Fatalf("Settings were not applied: %+v, %v", actual, err)
	}
	data, _ := os.ReadFile(filepath.Join(f.root, "wireless"))
	if !strings.Contains(string(data), "fixture-secret-never-return") {
		t.Fatal("Changing public settings changed an unmanaged password")
	}
	if _, err := os.Stat(filepath.Join(f.root, "lock")); !os.IsNotExist(err) {
		t.Fatal("Confirmed transaction left private backup files or locks")
	}
}

func TestUnconfirmedChangeRollsBackAfterSessionEnds(t *testing.T) {
	f := newFixture(t)
	f.delay = "2"
	before, _ := os.ReadFile(filepath.Join(f.root, "wireless"))
	_, err := f.Run(context.Background(), applyScript(Settings{Section: "wifi2g", SSID: "Unconfirmed"}, strings.Repeat("a", 32)))
	if err != nil {
		t.Fatal(err)
	}
	changed, _ := os.ReadFile(filepath.Join(f.root, "wireless"))
	if string(changed) == string(before) {
		t.Fatal("Test did not reach an applied change")
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		actual, _ := os.ReadFile(filepath.Join(f.root, "wireless"))
		_, pending := os.Stat(filepath.Join(f.root, "lock"))
		if string(actual) == string(before) && os.IsNotExist(pending) {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("Recovery did not restore the original configuration and clean up")
}

func TestWatchdogFailureAndStagedChangesStopWrites(t *testing.T) {
	for _, name := range []string{"fail-watchdog", "staged"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			before, _ := os.ReadFile(filepath.Join(f.root, "wireless"))
			if err := os.WriteFile(filepath.Join(f.root, name), before, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := f.Run(context.Background(), applyScript(Settings{Section: "wifi2g", SSID: "Should-not-apply"}, strings.Repeat("b", 32))); err == nil {
				t.Fatal("Failed precondition was accepted")
			}
			after, _ := os.ReadFile(filepath.Join(f.root, "wireless"))
			if string(before) != string(after) {
				t.Fatal("Failed precondition changed the persisted configuration")
			}
			if _, err := os.Stat(filepath.Join(f.root, "lock")); !os.IsNotExist(err) {
				t.Fatal("Failed preparation left a lock or backup")
			}
		})
	}
}
