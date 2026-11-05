package router

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
)

type fixtureRunner struct{ root string }

func newFixture(t *testing.T) *fixtureRunner {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexec \"$GLINET_TEST_BINARY\" -test.run=^TestRouterHelper$ -- start-stop-daemon \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "start-stop-daemon"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return &fixtureRunner{root: root}
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
	if len(args) < 3 {
		os.Exit(2)
	}
	args = args[2:]
	pidFile := ""
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-p" {
			pidFile = args[i+1]
		}
	}
	if len(args) == 0 || pidFile == "" {
		os.Exit(2)
	}
	if _, err := os.Stat(filepath.Join(root, "fail-watchdog")); err == nil {
		os.Exit(1)
	}
	command := exec.Command("/bin/sh", args[len(args)-1])
	command.Env = os.Environ()
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if command.Start() != nil {
		os.Exit(2)
	}
	if os.WriteFile(pidFile, []byte(strconv.Itoa(command.Process.Pid)), 0600) != nil {
		_ = command.Process.Kill()
		os.Exit(2)
	}
	_ = command.Process.Release()
	os.Exit(0)
}
