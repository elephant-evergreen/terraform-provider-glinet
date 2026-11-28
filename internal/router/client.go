package router

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/sync/singleflight"
)

var sectionName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
var packageName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type Client struct {
	runner     Runner
	mu         sync.RWMutex
	refresh    singleflight.Group
	protection ChangeProtection

	allowNetworkDisruption bool
	managementPort         int
	transactionLog         string
}

type ChangeProtection interface {
	Protect(context.Context, Runner, bool) error
}

func NewClient(runner Runner, protection ChangeProtection) *Client {
	return &Client{runner: runner, protection: protection}
}

// SetManagementPort records the SSH port used to reach the router. The SSH
// resource uses it to decide whether activating a change would close the
// management path the provider is connected through.
func (c *Client) SetManagementPort(port int) {
	c.managementPort = port
}

func (c *Client) ManagementPort() int {
	return c.managementPort
}

// AllowNetworkDisruption permits changes that can interrupt the network
// connection, such as a network, port, Wi-Fi, or GoodCloud change.
func (c *Client) AllowNetworkDisruption(allow bool) {
	c.allowNetworkDisruption = allow
}

func (c *Client) Verify(ctx context.Context) error {
	_, err := c.VerifyFirmware(ctx)
	return err
}

func (c *Client) VerifyFirmware(ctx context.Context) (SystemInfo, error) {
	if _, err := c.runner.Run(ctx, preflight); err != nil {
		return SystemInfo{}, operationError(err)
	}
	if err := c.verifyRuntimeFirewall(ctx); err != nil {
		return SystemInfo{}, err
	}
	info, err := c.System(ctx)
	if err != nil {
		return SystemInfo{}, err
	}
	return info, nil
}

const supportedFirmwareMinor = "4.11"

var supportedFirmware = regexp.MustCompile(`^4\.11\.[0-9]{1,2}$`)

func FirmwareAllowed(version string) bool {
	return supportedFirmware.MatchString(version)
}

func ValidSection(section string) bool {
	return len(section) <= 64 && sectionName.MatchString(section)
}

func validPackageName(name string) bool {
	return len(name) <= 64 && packageName.MatchString(name)
}

func ValidSSID(ssid string) bool {
	return len(ssid) >= 1 && len(ssid) <= 32 && utf8.ValidString(ssid) && !strings.ContainsFunc(ssid, unicode.IsControl)
}

func quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

var preflight = fmt.Sprintf(`set -eu
firmware="$(cat /etc/glversion)"
case "$firmware" in
  %s.[0-9]|%s.[0-9][0-9]) ;;
  *) exit 65 ;;
esac
test "$(ubus call system board | jsonfilter -e '@.model')" = 'GL.iNet GL-MT6000' || exit 65
test "$(uci -q get glconfig.general.mode)" = 'router' || exit 65
`, supportedFirmwareMinor, supportedFirmwareMinor)

func operationError(err error) error {
	var exit *ExitError
	if errors.As(err, &exit) {
		switch exit.Code {
		case 44:
			return errors.New("The named UCI section does not exist.")
		case 65:
			return errors.New("Unsupported router configuration. Use Flint 2 (GL-MT6000), stock firmware 4.11.x in router mode and the documented UCI section identities.")
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
		case 78:
			return errors.New("The router section is owned by another Terraform state. No configuration was changed; re-run the plan after checking which state manages the section.")
		}
	}
	return err
}

func (c *Client) Reboot(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.runner.Run(ctx, "sh -c '(sleep 1; /sbin/reboot) >/dev/null 2>&1 &'")
	return err
}

func (c *Client) CreateBackup(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.protection == nil {
		return fmt.Errorf("encrypted backups are not configured")
	}
	return c.protection.Protect(ctx, c.runner, false)
}
