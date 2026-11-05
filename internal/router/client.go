package router

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
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
			return errors.New("Unsupported router configuration. Use Flint 2 (GL-MT6000), stock firmware 4.11.0 in router mode and the documented UCI section identities.")
		case 73:
			return errors.New("Another change or recovery is pending. Wait for it to finish before applying again.")
		case 74:
			return errors.New("Uncommitted UCI changes exist. Commit or revert them outside Terraform before applying.")
		case 75:
			return errors.New("The change could not be confirmed before recovery started. Inspect the router before retrying.")
		case 76:
			return errors.New("Cannot arm automatic recovery. No configuration was changed. Inspect pending recovery before retrying.")
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

func settingsConfiguration(settings Settings) Configuration {
	return Configuration{{Package: "wireless", Name: settings.Section, Type: "wifi-iface", Options: map[string]string{"ssid": settings.SSID, "disabled": flag(settings.Disabled), "hidden": flag(settings.Hidden), "isolate": flag(settings.Isolate)}}}
}

func (c *Client) ReadSettingsStable(ctx context.Context, section string) (Settings, error) {
	if !ValidSection(section) {
		return Settings{}, errors.New("Provide an existing named wireless AP section.")
	}
	config := settingsConfiguration(Settings{Section: section})
	config[0].Options["mode"] = "ap"
	actual, err := c.StableConfiguration(ctx, config)
	if err != nil {
		return Settings{}, err
	}
	if actual[0].Missing {
		return Settings{}, ErrNotFound
	}
	values := actual[0].Options
	if values["mode"] != "ap" || !ValidSSID(values["ssid"]) {
		return Settings{}, errors.New("The router returned unsupported Wi-Fi AP settings.")
	}
	for _, name := range []string{"disabled", "hidden", "isolate"} {
		if values[name] != "0" && values[name] != "1" {
			return Settings{}, errors.New("The router returned an invalid Wi-Fi flag.")
		}
	}
	return Settings{Section: section, SSID: values["ssid"], Disabled: values["disabled"] == "1", Hidden: values["hidden"] == "1", Isolate: values["isolate"] == "1"}, nil
}

func (c *Client) Write(ctx context.Context, settings Settings) error {
	if !ValidSection(settings.Section) || !ValidSSID(settings.SSID) {
		return errors.New("Provide a named UCI section and a UTF-8 SSID of 1–32 bytes without control characters.")
	}
	if _, err := c.Read(ctx, settings.Section); err != nil {
		return err
	}
	return c.WriteConfiguration(ctx, settingsConfiguration(settings), nil)
}
