//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcceptanceHardware(t *testing.T) {
	requireAcceptance(t)
	client := acceptanceClient(t)

	t.Run("preflight", func(t *testing.T) {
		ctx := context.Background()
		info, err := client.VerifyFirmware(ctx)
		if err != nil {
			t.Fatalf("Preflight failed: %v", err)
		}
		if info.Mode != "router" {
			t.Fatalf("Router mode is %q, want router", info.Mode)
		}
		assertAcceptanceClean(t, ctx)
	})

	t.Run("firewall_redirect", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: acceptanceFactories(),
			Steps: []resource.TestStep{
				{Config: acceptanceRedirect("65530"), Check: resource.TestCheckResourceAttr("glinet_firewall_redirect.acceptance", "dest_port", "65530")},
				{Config: acceptanceRedirect("65531"), Check: resource.TestCheckResourceAttr("glinet_firewall_redirect.acceptance", "dest_port", "65531")},
				{
					ResourceName:            "glinet_firewall_redirect.acceptance",
					ImportState:             true,
					ImportStateVerify:       true,
					ImportStateVerifyIgnore: []string{"delete_behavior"},
				},
				{Config: acceptanceRedirect("65531")},
			},
		})
	})

	t.Run("openvpn_server", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: acceptanceFactories(),
			Steps: []resource.TestStep{
				{Config: acceptanceOpenVPNServer("1195"), Check: resource.TestCheckResourceAttr("glinet_openvpn_server.acceptance", "enabled", "false")},
				{Config: acceptanceOpenVPNServer("1196"), Check: resource.TestCheckResourceAttr("glinet_openvpn_server.acceptance", "port", "1196")},
				{
					ResourceName:            "glinet_openvpn_server.acceptance",
					ImportState:             true,
					ImportStateVerify:       true,
					ImportStateVerifyIgnore: []string{"delete_behavior"},
				},
				{Config: acceptanceOpenVPNServer("1196")},
			},
		})
	})

	t.Run("samba_share", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: acceptanceFactories(),
			Steps: []resource.TestStep{
				{Config: acceptanceSambaShare("acceptance"), Check: resource.TestCheckResourceAttr("glinet_samba_share.acceptance", "share_name", "acceptance")},
				{Config: acceptanceSambaShare("acceptance-2"), Check: resource.TestCheckResourceAttr("glinet_samba_share.acceptance", "share_name", "acceptance-2")},
				{
					ResourceName:            "glinet_samba_share.acceptance",
					ImportState:             true,
					ImportStateVerify:       true,
					ImportStateVerifyIgnore: []string{"delete_behavior"},
				},
				{Config: acceptanceSambaShare("acceptance-2")},
			},
		})
	})

	t.Run("firewall_zone", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: acceptanceFactories(),
			Steps: []resource.TestStep{
				{Config: acceptanceZone("REJECT"), Check: resource.TestCheckResourceAttr("glinet_firewall_zone.acceptance", "input", "REJECT")},
				{Config: acceptanceZone("DROP"), Check: resource.TestCheckResourceAttr("glinet_firewall_zone.acceptance", "input", "DROP")},
				{
					ResourceName:            "glinet_firewall_zone.acceptance",
					ImportState:             true,
					ImportStateVerify:       true,
					ImportStateVerifyIgnore: []string{"delete_behavior"},
				},
				{Config: acceptanceZone("DROP")},
			},
		})
	})

	t.Run("rollback", acceptanceRollback)

	t.Run("reboot", func(t *testing.T) {
		if os.Getenv("GLINET_ACCEPTANCE_REBOOT") != "1" {
			t.Skip("set GLINET_ACCEPTANCE_REBOOT=1 to reboot the router as part of the acceptance run")
		}
		ctx := context.Background()
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

func acceptanceFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"glinet": providerserver.NewProtocol6WithError(New("acceptance")()),
	}
}

func acceptanceClient(t *testing.T) *router.Client {
	t.Helper()
	transport := acceptanceRunner(t)
	protection, err := router.NewProtection(os.Getenv("GLINET_BACKUP_DIRECTORY"), os.Getenv("GLINET_PRIVATE_KEY_FILE"), os.Getenv("GLINET_HOST"), acceptancePort(t))
	if err != nil {
		t.Fatal(err)
	}
	return router.NewClient(transport, protection)
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
			t.Fatalf("GLINET_PORT must be a decimal integer between 1 and 65535")
		}
		return port
	}
	return 22
}

func assertAcceptanceClean(t *testing.T, ctx context.Context) {
	t.Helper()
	runner := acceptanceRunner(t)
	output, err := runner.Run(ctx, "uci changes | wc -l")
	if err != nil {
		t.Fatalf("Cannot read pending changes: %v", err)
	}
	if strings.TrimSpace(string(output)) != "0" {
		t.Fatalf("The router has %s pending UCI changes", strings.TrimSpace(string(output)))
	}
	output, err = runner.Run(ctx, "test -e /root/.terraform-provider-glinet-configuration && echo busy || echo clean")
	if err != nil {
		t.Fatalf("Cannot read the transaction state: %v", err)
	}
	if strings.TrimSpace(string(output)) != "clean" {
		t.Fatal("A router-side transaction is still present")
	}
}

func acceptanceRollback(t *testing.T) {
	ctx := context.Background()
	client := acceptanceClient(t)
	name := "Acceptance-Rollback"
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
	defer func() {
		_, _ = injector.Run(context.Background(), "iptables -t filter -D INPUT -p tcp --dport 65500 -j ACCEPT 2>/dev/null || true")
	}()
	applied := make(chan error, 1)
	go func() {
		applied <- client.ApplySections(context.Background(), router.Configuration{changed}, nil)
	}()
	injected := false
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-applied:
			t.Fatalf("The apply finished before the injected rule took effect: %v", err)
		default:
		}
		output, err := injector.Run(ctx, "test -f /root/.terraform-provider-glinet-configuration/ready && echo ready")
		if err == nil && strings.TrimSpace(string(output)) == "ready" {
			if _, err := injector.Run(ctx, "iptables -t filter -A INPUT -p tcp --dport 65500 -j ACCEPT"); err != nil {
				t.Fatalf("Cannot inject the firewall rule: %v", err)
			}
			injected = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !injected {
		t.Fatal("The apply never reached the confirmation phase")
	}
	err = <-applied
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
	output, err := injector.Run(ctx, "iptables -t filter -S INPUT 2>/dev/null | grep -c 65500 || true")
	if err != nil {
		t.Fatalf("Cannot inspect the injected rule: %v", err)
	}
	if strings.TrimSpace(string(output)) != "0" {
		t.Fatalf("The injected rule survived the rollback")
	}
}

func acceptanceRedirect(port string) string {
	return fmt.Sprintf(`
resource "glinet_firewall_redirect" "acceptance" {
  delete_behavior = "delete"
  name            = "Acceptance-Redirect"
  src             = "wan"
  src_dport       = "65530"
  dest            = "lan"
  dest_ip         = "192.0.2.10"
  dest_port       = %q
  proto           = "tcp"
  target          = "DNAT"
  enabled         = true
}
`, port)
}

func acceptanceOpenVPNServer(port string) string {
	return fmt.Sprintf(`
resource "glinet_openvpn_server" "acceptance" {
  delete_behavior = "delete"
  name            = "acceptance_server"
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
`, port)
}

func acceptanceSambaShare(name string) string {
	return fmt.Sprintf(`
resource "glinet_samba_share" "acceptance" {
  delete_behavior = "delete"
  section         = "acceptance_share"
  share_name      = %q
  path            = "/tmp"
}
`, name)
}

func acceptanceZone(input string) string {
	return fmt.Sprintf(`
resource "glinet_firewall_zone" "acceptance" {
  delete_behavior = "delete"
  name            = "acceptance_zone"
  input           = %q
}
`, input)
}
