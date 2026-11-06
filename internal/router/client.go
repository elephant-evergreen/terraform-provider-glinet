package router

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

var sectionName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

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

func operationError(err error) error {
	var exit *ExitError
	if errors.As(err, &exit) {
		switch exit.Code {
		case 44:
			return errors.New("The named UCI section does not exist.")
		case 65:
			return errors.New("Unsupported router configuration. Use Flint 2 (GL-MT6000), stock firmware 4.11.0 in router mode and the documented UCI section identities.")
		case 73:
			return errors.New("Another change or recovery is pending. Wait for it to finish before applying again.")
		case 74:
			return errors.New("Uncommitted UCI changes exist. Commit or revert them outside Terraform before applying.")
		case 75:
			return errors.New("The change could not be confirmed before recovery started. Inspect the router before retrying.")
		case 77:
			return errors.New("The router selected rollback or canceled the transaction. Inspect recovery and connectivity before retrying.")
		case 76:
			return errors.New("Cannot arm automatic recovery. No configuration was changed. Inspect pending recovery before retrying.")
		}
	}
	return err
}
