//go:build acceptance

package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAcceptanceHardware(t *testing.T) {
	requireAcceptance(t)
	client := acceptanceClient(t)
	runner := acceptanceRunner(t)
	ctx := context.Background()
	info, err := client.VerifyFirmware(ctx)
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}
	if info.Mode != "router" {
		t.Fatalf("Router mode is %q, want router", info.Mode)
	}
	assertAcceptanceClean(t, ctx)

	nonce := acceptanceNonce(t)
	redirect := "Acceptance-Redirect-" + nonce
	openvpn := "acceptance_server_" + nonce
	samba := "acceptance_share_" + nonce
	zone := "acceptance_zone_" + nonce

	t.Run("firewall_redirect", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: acceptanceFactories(),
			CheckDestroy:             acceptanceGone(runner, "firewall", redirect),
			Steps: []resource.TestStep{
				{
					Config: acceptanceRedirect(redirect, "65530"),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("glinet_firewall_redirect.acceptance", "dest_port", "65530"),
						acceptanceFirewallValue(runner, redirect, "dest_port", "65530"),
					),
				},
				{
					Config: acceptanceRedirect(redirect, "65531"),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("glinet_firewall_redirect.acceptance", "dest_port", "65531"),
						acceptanceFirewallValue(runner, redirect, "dest_port", "65531"),
					),
				},
				{
					ResourceName:            "glinet_firewall_redirect.acceptance",
					ImportState:             true,
					ImportStateVerify:       true,
					ImportStateVerifyIgnore: []string{"delete_behavior"},
				},
				{Config: acceptanceRedirect(redirect, "65531")},
			},
		})
	})

	t.Run("openvpn_server", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: acceptanceFactories(),
			CheckDestroy:             acceptanceGone(runner, "openvpn", openvpn),
			Steps: []resource.TestStep{
				{
					Config: acceptanceOpenVPNServer(openvpn, "1195"),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("glinet_openvpn_server.acceptance", "enabled", "false"),
						acceptanceObservation(runner, "openvpn", openvpn, "port", "1195"),
					),
				},
				{
					Config: acceptanceOpenVPNServer(openvpn, "1196"),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("glinet_openvpn_server.acceptance", "port", "1196"),
						acceptanceObservation(runner, "openvpn", openvpn, "port", "1196"),
					),
				},
				{
					ResourceName:            "glinet_openvpn_server.acceptance",
					ImportState:             true,
					ImportStateVerify:       true,
					ImportStateVerifyIgnore: []string{"delete_behavior"},
				},
				{Config: acceptanceOpenVPNServer(openvpn, "1196")},
			},
		})
	})

	t.Run("samba_share", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: acceptanceFactories(),
			CheckDestroy:             acceptanceGone(runner, "samba4", samba),
			Steps: []resource.TestStep{
				{
					Config: acceptanceSambaShare(samba, "acceptance-"+nonce),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("glinet_samba_share.acceptance", "share_name", "acceptance-"+nonce),
						acceptanceObservation(runner, "samba4", samba, "name", "acceptance-"+nonce),
					),
				},
				{
					Config: acceptanceSambaShare(samba, "acceptance-2-"+nonce),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("glinet_samba_share.acceptance", "share_name", "acceptance-2-"+nonce),
						acceptanceObservation(runner, "samba4", samba, "name", "acceptance-2-"+nonce),
					),
				},
				{
					ResourceName:            "glinet_samba_share.acceptance",
					ImportState:             true,
					ImportStateVerify:       true,
					ImportStateVerifyIgnore: []string{"delete_behavior"},
				},
				{Config: acceptanceSambaShare(samba, "acceptance-2-"+nonce)},
			},
		})
	})

	t.Run("firewall_zone", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: acceptanceFactories(),
			CheckDestroy:             acceptanceGone(runner, "firewall", zone),
			Steps: []resource.TestStep{
				{
					Config: acceptanceZone(zone, "REJECT"),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("glinet_firewall_zone.acceptance", "input", "REJECT"),
						acceptanceFirewallValue(runner, zone, "name", zone),
					),
				},
				{
					Config: acceptanceZone(zone, "DROP"),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("glinet_firewall_zone.acceptance", "input", "DROP"),
						acceptanceFirewallValue(runner, zone, "input", "DROP"),
					),
				},
				{
					ResourceName:            "glinet_firewall_zone.acceptance",
					ImportState:             true,
					ImportStateVerify:       true,
					ImportStateVerifyIgnore: []string{"delete_behavior"},
				},
				{Config: acceptanceZone(zone, "DROP")},
			},
		})
	})

	t.Run("service_activation", acceptanceServiceActivation)

	t.Run("rollback", acceptanceRollback)

	t.Run("reboot", func(t *testing.T) {
		if os.Getenv("GLINET_ACCEPTANCE_REBOOT") != "1" {
			t.Skip("set GLINET_ACCEPTANCE_REBOOT=1 to reboot the router as part of the acceptance run")
		}
		if err := client.Reboot(ctx); err != nil {
			t.Fatalf("Reboot failed: %v", err)
		}
		deadline := time.Now().Add(5 * time.Minute)
		for time.Now().Before(deadline) {
			time.Sleep(5 * time.Second)
			if _, err := client.VerifyFirmware(context.Background()); err == nil {
				assertAcceptanceClean(t, context.Background())
				return
			}
		}
		t.Fatal("The router did not return after the reboot")
	})

	t.Run("clean", func(t *testing.T) {
		assertAcceptanceClean(t, context.Background())
	})
}

func requireAcceptance(t *testing.T) {
	t.Helper()
	if os.Getenv("GLINET_ACCEPTANCE") != "1" {
		t.Skip("Set GLINET_ACCEPTANCE=1 and the GLINET_* connection variables to run the hardware acceptance suite against a dedicated router.")
	}
	for _, name := range []string{"GLINET_HOST", "GLINET_PRIVATE_KEY_FILE", "GLINET_KNOWN_HOSTS_FILE"} {
		if os.Getenv(name) == "" {
			t.Fatalf("%s is required for the acceptance suite", name)
		}
	}
	t.Setenv("TF_ACC", "1")
	t.Setenv("GLINET_BACKUP_DIRECTORY", t.TempDir())
}

func acceptanceNonce(t *testing.T) string {
	t.Helper()
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(buffer)
}

func acceptanceFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"glinet": providerserver.NewProtocol6WithError(New("acceptance")()),
	}
}

func acceptanceClient(t *testing.T) *router.Client {
	t.Helper()
	protection, err := router.NewProtection(os.Getenv("GLINET_BACKUP_DIRECTORY"), os.Getenv("GLINET_PRIVATE_KEY_FILE"), os.Getenv("GLINET_HOST"), acceptancePort(t))
	if err != nil {
		t.Fatal(err)
	}
	return router.NewClient(acceptanceRunner(t), protection)
}

func acceptanceRunner(t *testing.T) *router.SSH {
	t.Helper()
	runner, err := router.NewSSH(os.Getenv("GLINET_HOST"), acceptancePort(t), os.Getenv("GLINET_PRIVATE_KEY_FILE"), os.Getenv("GLINET_KNOWN_HOSTS_FILE"))
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func acceptancePort(t *testing.T) int {
	t.Helper()
	if value := os.Getenv("GLINET_PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			t.Fatal("GLINET_PORT must be a decimal integer between 1 and 65535")
		}
		return port
	}
	return 22
}

func acceptanceQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func acceptanceObservation(runner *router.SSH, pkg, name, option, value string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		script := "uci show " + pkg + " 2>/dev/null | grep -F " + acceptanceQuote(name) + " | grep -c " + acceptanceQuote(option+"='"+value+"'") + " || true"
		output, err := runner.Run(context.Background(), script)
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(output)) == "0" {
			return fmt.Errorf("%s.%s does not show %s=%s on the router", pkg, name, option, value)
		}
		return nil
	}
}

func acceptanceFirewallValue(runner *router.SSH, name, option, value string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		script := "name=" + acceptanceQuote(name) + "\n" +
			"section=$(uci show firewall 2>/dev/null | awk -F. -v want=\"name='\"$name\"'\" '$NF==want {print $2; exit}')\n" +
			"[ -n \"$section\" ] || exit 1\n" +
			"uci -q get firewall.$section." + option + "\n"
		output, err := runner.Run(context.Background(), script)
		if err != nil {
			return fmt.Errorf("%s is not on the router: %w", name, err)
		}
		if strings.TrimSpace(string(output)) != value {
			return fmt.Errorf("%s.%s = %q on the router, want %q", name, option, strings.TrimSpace(string(output)), value)
		}
		return nil
	}
}

func acceptanceGone(runner *router.SSH, pkg, name string) func(*terraform.State) error {
	return func(*terraform.State) error {
		output, err := runner.Run(context.Background(), "uci show "+pkg+" 2>/dev/null | grep -c "+acceptanceQuote(name)+" || true")
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(output)) != "0" {
			return fmt.Errorf("%s.%s remains on the router after destroy", pkg, name)
		}
		return nil
	}
}

func assertAcceptanceClean(t *testing.T, ctx context.Context) {
	t.Helper()
	runner := acceptanceRunner(t)
	for _, check := range []struct {
		name string
		cmd  string
		want string
	}{
		{"pending UCI changes", "uci changes | wc -l", "0"},
		{"transaction", "test -e /root/.terraform-provider-glinet-configuration && echo busy || echo clean", "clean"},
		{"Wi-Fi lock", "test -e /tmp/terraform-provider-glinet-wifi && echo busy || echo clean", "clean"},
		{"recovery service", "test -e /etc/init.d/terraform-glinet-recovery && echo present || echo clean", "clean"},
		{"recovery links", "ls /etc/rc.d/S??terraform-glinet-recovery /etc/rc.d/K??terraform-glinet-recovery 2>/dev/null | wc -l", "0"},
	} {
		output, err := runner.Run(ctx, check.cmd)
		if err != nil {
			t.Fatalf("Cannot check %s: %v", check.name, err)
		}
		if strings.TrimSpace(string(output)) != check.want {
			t.Fatalf("%s is not clean: %s", check.name, strings.TrimSpace(string(output)))
		}
	}
}

func acceptanceServiceActivation(t *testing.T) {
	ctx := context.Background()
	client := acceptanceClient(t)
	runner := acceptanceRunner(t)
	section := router.Section{Package: "system", Type: "timeserver", Name: "ntp"}
	actual, ok, err := client.ReadSection(ctx, section)
	if err != nil || !ok {
		t.Fatalf("Cannot read the NTP section: %v", err)
	}
	original := acceptanceCloneSection(actual)
	defer func() {
		if err := client.ApplySections(context.Background(), router.Configuration{original}, nil); err != nil {
			t.Errorf("Cannot restore the NTP section: %v", err)
		}
	}()
	changed := acceptanceCloneSection(actual)
	if changed.Lists == nil {
		changed.Lists = map[string][]string{}
	}
	changed.Lists["server"] = append(changed.Lists["server"], "192.0.2.1")
	before := acceptancePID(t, runner, "chronyd")
	if err := client.ApplySections(ctx, router.Configuration{changed}, nil); err != nil {
		t.Fatalf("Cannot change the NTP configuration: %v", err)
	}
	if after := acceptanceWaitPID(t, runner, "chronyd", before, 30*time.Second); after == "" {
		t.Fatal("chronyd did not restart after the NTP change")
	}
}

func acceptanceCloneSection(section router.Section) router.Section {
	clone := section
	clone.Options = map[string]string{}
	for key, value := range section.Options {
		clone.Options[key] = value
	}
	clone.Lists = map[string][]string{}
	for key, values := range section.Lists {
		clone.Lists[key] = append([]string(nil), values...)
	}
	return clone
}

func acceptancePID(t *testing.T, runner *router.SSH, name string) string {
	t.Helper()
	output, err := runner.Run(context.Background(), "pidof "+name+" 2>/dev/null | awk '{print $1}'")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func acceptanceWaitPID(t *testing.T, runner *router.SSH, name, before string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pid := acceptancePID(t, runner, name); pid != "" && pid != before {
			return pid
		}
		time.Sleep(500 * time.Millisecond)
	}
	return ""
}

func acceptanceRollback(t *testing.T) {
	ctx := context.Background()
	client := acceptanceClient(t)
	runner := acceptanceRunner(t)
	name := "Acceptance-Rollback-" + acceptanceNonce(t)
	section := router.Section{
		Package: "firewall", Type: "redirect", Name: "@redirect[0]",
		Match: map[string]string{"name": name},
		Options: map[string]string{
			"name": name, "target": "DNAT", "src": "wan", "src_dport": "65530",
			"dest": "lan", "dest_ip": "192.0.2.10", "dest_port": "65530",
			"proto": "tcp", "enabled": "1", router.ManagedOption: "1",
		},
	}
	if err := client.ApplySections(ctx, router.Configuration{section}, nil); err != nil {
		t.Fatalf("Cannot create the rollback redirect: %v", err)
	}
	actual, ok, err := client.ReadSection(ctx, section)
	if err != nil || !ok {
		t.Fatalf("Cannot read the rollback redirect: %v", err)
	}
	defer func() {
		deadline := time.Now().Add(7 * time.Minute)
		for time.Now().Before(deadline) {
			output, err := runner.Run(context.Background(), "test -e /root/.terraform-provider-glinet-configuration && echo busy || echo clean")
			if err != nil || strings.TrimSpace(string(output)) == "clean" {
				break
			}
			time.Sleep(5 * time.Second)
		}
		cleanup := router.Section{Package: "firewall", Name: actual.Name, Type: "redirect", Absent: true}
		if err := client.ApplySections(context.Background(), router.Configuration{cleanup}, nil); err != nil {
			t.Errorf("Cannot remove the rollback redirect: %v", err)
		}
	}()
	changed := actual
	changed.Options = map[string]string{}
	for key, value := range actual.Options {
		changed.Options[key] = value
	}
	changed.Options["dest_port"] = "65531"
	changed.Match = nil

	injector := acceptanceRunner(t)
	control := acceptanceRunner(t)
	stopFile := "/tmp/terraform-glinet-acceptance-stop-" + acceptanceNonce(t)
	var stopOnce sync.Once
	stopInjector := func() {
		stopOnce.Do(func() {
			_, _ = control.Run(context.Background(), "touch "+acceptanceQuote(stopFile))
		})
	}
	defer stopInjector()
	injected := make(chan error, 1)
	go func() {
		script := "set -u\nstop=" + acceptanceQuote(stopFile) + "\n" +
			"while [ ! -f \"$stop\" ]; do\n" +
			"  iptables -t filter -C INPUT -p tcp --dport 65500 -j ACCEPT 2>/dev/null || iptables -t filter -A INPUT -p tcp --dport 65500 -j ACCEPT\n" +
			"  sleep 0.2\n" +
			"done\n" +
			"while iptables -t filter -D INPUT -p tcp --dport 65500 -j ACCEPT 2>/dev/null; do :; done\n" +
			"rm -f \"$stop\"\n"
		_, err := injector.Run(context.Background(), script)
		injected <- err
	}()
	err = client.ApplySections(context.Background(), router.Configuration{changed}, nil)
	stopInjector()
	select {
	case <-injected:
	case <-time.After(30 * time.Second):
		t.Fatal("The firewall injector did not stop")
	}
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("Expected a rolled-back apply, got %v", err)
	}
	restored, ok, err := client.ReadSection(ctx, section)
	if err != nil || !ok {
		t.Fatalf("Cannot read the restored redirect: %v", err)
	}
	if restored.Options["dest_port"] != "65530" {
		t.Fatalf("Rollback left dest_port %q", restored.Options["dest_port"])
	}
	for attempt := 0; attempt < 5; attempt++ {
		if _, err := runner.Run(ctx, "iptables -t filter -D INPUT -p tcp --dport 65500 -j ACCEPT 2>/dev/null || true"); err != nil {
			t.Fatalf("Cannot clean up the injected rule: %v", err)
		}
		output, err := runner.Run(ctx, "iptables -t filter -S INPUT 2>/dev/null | grep -c 65500 || true")
		if err != nil {
			t.Fatalf("Cannot inspect the injected rule: %v", err)
		}
		if strings.TrimSpace(string(output)) == "0" {
			return
		}
	}
}

func acceptanceRedirect(name, port string) string {
	return fmt.Sprintf(`
resource "glinet_firewall_redirect" "acceptance" {
  delete_behavior = "delete"
  name            = %q
  src             = "wan"
  src_dport       = "65530"
  dest            = "lan"
  dest_ip         = "192.0.2.10"
  dest_port       = %q
  proto           = "tcp"
  target          = "DNAT"
  enabled         = true
}
`, name, port)
}

func acceptanceOpenVPNServer(name, port string) string {
	return fmt.Sprintf(`
resource "glinet_openvpn_server" "acceptance" {
  delete_behavior = "delete"
  name            = %q
  enabled         = false
  port            = %s
  proto           = "udp"
  dev             = "tun"
  server          = "10.99.0.0 255.255.255.0"
  ca              = ""
  cert            = ""
  key             = ""
  dh              = ""
}
`, name, port)
}

func acceptanceSambaShare(section, name string) string {
	return fmt.Sprintf(`
resource "glinet_samba_share" "acceptance" {
  delete_behavior = "delete"
  section         = %q
  share_name      = %q
  path            = "/tmp"
}
`, section, name)
}

func acceptanceZone(name, input string) string {
	return fmt.Sprintf(`
resource "glinet_firewall_zone" "acceptance" {
  delete_behavior = "delete"
  name            = %q
  input           = %q
}
`, name, input)
}
