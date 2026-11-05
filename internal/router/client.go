package router

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrNotFound = errors.New("The named Wi-Fi AP section does not exist. This version manages settings on existing named AP sections only.")
var sectionName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type Settings struct {
	Section  string
	SSID     string
	Disabled bool
	Hidden   bool
	Isolate  bool
}

type Client struct {
	runner Runner
	mu     sync.Mutex
}

func NewClient(runner Runner) *Client { return &Client{runner: runner} }

func (c *Client) Verify(ctx context.Context) error {
	_, err := c.runner.Run(ctx, preflight)
	return operationError(err)
}

func ValidSection(section string) bool {
	return len(section) <= 64 && sectionName.MatchString(section)
}

func ValidSSID(ssid string) bool {
	return len(ssid) >= 1 && len(ssid) <= 32 && utf8.ValidString(ssid) && !strings.ContainsFunc(ssid, unicode.IsControl)
}

func quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

const preflight = `set -eu
test "$(cat /etc/glversion)" = '4.11.0' || exit 65
test "$(ubus call system board | jsonfilter -e '@.model')" = 'GL.iNet GL-MT6000' || exit 65
test "$(uci -q get glconfig.general.mode)" = 'router' || exit 65
`

func sectionCheck(section string) string {
	return "test \"$(uci -q get " + quote("wireless."+section) + ")\" = 'wifi-iface' || exit 44\n" +
		"test \"$(uci -q get " + quote("wireless."+section+".mode") + ")\" = 'ap' || exit 65\n"
}

func operationError(err error) error {
	var exit *ExitError
	if errors.As(err, &exit) {
		switch exit.Code {
		case 44:
			return ErrNotFound
		case 65:
			return errors.New("Unsupported router configuration. Only Flint 2 (GL-MT6000), stock firmware 4.11.0, router mode, and existing AP sections are supported.")
		case 73:
			return errors.New("Another Wi-Fi change or recovery is pending. Wait for it to finish before applying again.")
		case 74:
			return errors.New("Uncommitted UCI changes exist. Commit or revert them outside Terraform before applying.")
		case 75:
			return errors.New("The change could not be confirmed before recovery started. Inspect the router before retrying.")
		case 76:
			return errors.New("Cannot arm automatic recovery. No Wi-Fi settings were changed.")
		}
	}
	return err
}

func (c *Client) Read(ctx context.Context, section string) (Settings, error) {
	if !ValidSection(section) {
		return Settings{}, errors.New("section must be a named UCI section using 1–64 letters, digits, or underscores.")
	}
	script := preflight + sectionCheck(section)
	for _, name := range []string{"ssid", "disabled", "hidden", "isolate"} {
		fallback := "0"
		if name == "ssid" {
			fallback = ""
		}
		script += "printf '%s\\000' \"$(uci -q get " + quote("wireless."+section+"."+name) + " || printf '%s' " + quote(fallback) + ")\"\n"
	}
	output, err := c.runner.Run(ctx, script)
	if err != nil {
		return Settings{}, operationError(err)
	}
	parts := bytes.Split(output, []byte{0})
	if len(parts) != 5 || len(parts[4]) != 0 || !ValidSSID(string(parts[0])) {
		return Settings{}, errors.New("The router returned invalid Wi-Fi settings.")
	}
	for _, value := range parts[1:4] {
		if string(value) != "0" && string(value) != "1" {
			return Settings{}, errors.New("The router returned an invalid Wi-Fi flag.")
		}
	}
	return Settings{Section: section, SSID: string(parts[0]), Disabled: string(parts[1]) == "1", Hidden: string(parts[2]) == "1", Isolate: string(parts[3]) == "1"}, nil
}

func flag(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func applyScript(settings Settings, token string) string {
	dir := "/tmp/terraform-provider-glinet-wifi"
	watchdog := `#!/bin/sh
set -eu
dir='/tmp/terraform-provider-glinet-wifi'
sleep 120 &
child=$!
trap 'kill "$child" 2>/dev/null || true; exit 0' TERM INT
touch "$dir/armed"
wait "$child" || exit 0
if mkdir "$dir/decision" 2>/dev/null; then
  cp "$dir/wireless" /etc/config/wireless
  uci revert wireless
  wifi reload
fi
rm -rf "$dir"
`
	script := preflight + sectionCheck(settings.Section) +
		"pending=$(uci changes wireless)\ntest -z \"$pending\" || exit 74\n" +
		"umask 077\nmkdir " + quote(dir) + " 2>/dev/null || exit 73\n" +
		"armed=0\ntrap 'if [ \"$armed\" = 0 ]; then rm -rf " + quote(dir) + "; fi' EXIT\n" +
		"printf '%s' " + quote(token) + " > " + quote(dir+"/token") + "\n" +
		"cp /etc/config/wireless " + quote(dir+"/wireless") + "\n" +
		"printf '%s' " + quote(watchdog) + " > " + quote(dir+"/rollback") + "\n" +
		"start-stop-daemon -S -b -m -p " + quote(dir+"/pid") + " -x /bin/sh -- " + quote(dir+"/rollback") + "\n" +
		"attempt=0\nwhile [ ! -f " + quote(dir+"/armed") + " ]; do\n  attempt=$((attempt+1))\n  [ \"$attempt\" -lt 5 ] || exit 76\n  sleep 1\ndone\narmed=1\n"
	for _, pair := range [][2]string{{"ssid", settings.SSID}, {"disabled", flag(settings.Disabled)}, {"hidden", flag(settings.Hidden)}, {"isolate", flag(settings.Isolate)}} {
		script += "uci set " + quote("wireless."+settings.Section+"."+pair[0]+"="+pair[1]) + "\n"
	}
	return script + "uci commit wireless\nwifi reload\ntouch " + quote(dir+"/applied") + "\n"
}

func confirmScript(token string) string {
	dir := "/tmp/terraform-provider-glinet-wifi"
	return preflight +
		"test \"$(cat " + quote(dir+"/token") + ")\" = " + quote(token) + " || exit 75\n" +
		"test -f " + quote(dir+"/applied") + " || exit 75\n" +
		"mkdir " + quote(dir+"/decision") + " 2>/dev/null || exit 75\n" +
		"start-stop-daemon -K -p " + quote(dir+"/pid") + "\n" +
		"rm -rf " + quote(dir) + "\n"
}

func (c *Client) Write(ctx context.Context, settings Settings) error {
	if !ValidSection(settings.Section) || !ValidSSID(settings.SSID) {
		return errors.New("Provide a named UCI section and a UTF-8 SSID of 1–32 bytes without control characters.")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := c.Read(ctx, settings.Section)
	if err != nil || current == settings {
		return err
	}
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return errors.New("Cannot initialize a Wi-Fi change identifier.")
	}
	token := hex.EncodeToString(tokenBytes)
	_, applyErr := c.runner.Run(ctx, applyScript(settings, token))
	if applyErr != nil {
		var exit *ExitError
		if errors.As(applyErr, &exit) {
			return operationError(applyErr)
		}
	}
	recovery, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for {
		actual, readErr := c.Read(recovery, settings.Section)
		if readErr == nil && actual == settings {
			_, err = c.runner.Run(recovery, confirmScript(token))
			return operationError(err)
		}
		select {
		case <-recovery.Done():
			return fmt.Errorf("Wi-Fi change was not confirmed. The router recovery timer restores the previous wireless configuration after 120 seconds. Inspect connectivity before retrying: %w", recovery.Err())
		case <-time.After(time.Second):
		}
	}
}
