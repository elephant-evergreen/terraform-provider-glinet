package router

import (
	"strings"
	"testing"
)

func TestServiceForSections(t *testing.T) {
	cases := []struct {
		section Section
		want    string
		ok      bool
	}{
		{Section{Package: "samba4", Name: "samba"}, "samba4", true},
		{Section{Package: "openvpn", Name: "sample_server"}, "openvpn", true},
		{Section{Package: "tailscale", Name: "settings"}, "tailscale", true},
		{Section{Package: "sqm", Name: "wan"}, "sqm", true},
		{Section{Package: "adguardhome", Name: "config"}, "adguardhome", true},
		{Section{Package: "minidlna", Name: "config"}, "minidlna", true},
		{Section{Package: "upnpd", Name: "config"}, "miniupnpd", true},
		{Section{Package: "zerotier", Name: "gl"}, "zerotier", true},
		{Section{Package: "tor", Name: "global"}, "tor", true},
		{Section{Package: "system", Name: "ntp", Type: "timeserver"}, "chronyd", true},
		{Section{Package: "system", Name: "ntp", Type: "system"}, "", false},
		{Section{Package: "network", Name: "lan"}, "", false},
	}
	for _, test := range cases {
		service, ok := serviceFor(test.section)
		if ok != test.ok || service.Name != test.want {
			t.Errorf("serviceFor(%s.%s) = %q, %v; want %q, %v", test.section.Package, test.section.Name, service.Name, ok, test.want, test.ok)
		}
	}
}

func TestServiceActionScript(t *testing.T) {
	script := ntpService.script()
	for _, want := range []string{
		"/etc/init.d/chronyd restart",
		`[ "$(uci -q get system.ntp.enabled)" = 1 ]`,
		"pidof chronyd",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("chronyd script does not contain %q:\n%s", want, script)
		}
	}
	if strings.Contains(sambaService.script(), "pidof") {
		t.Fatal("samba does not declare a process check")
	}
}

func TestConfigurationReloadRunsServicesOnce(t *testing.T) {
	plan := reloadPlan{Firewall: true}
	addService(&plan, ntpService)
	addService(&plan, sambaService)
	addService(&plan, ntpService)
	script := configurationReload(plan)
	if strings.Count(script, "/etc/init.d/chronyd restart") != 1 {
		t.Fatalf("service reload was not deduplicated:\n%s", script)
	}
	if !strings.Contains(script, "/etc/init.d/samba4 restart") {
		t.Fatalf("samba reload missing:\n%s", script)
	}
	if strings.Index(script, "/etc/init.d/firewall restart") > strings.Index(script, "/etc/init.d/chronyd restart") {
		t.Fatalf("services ran before the firewall reload:\n%s", script)
	}
}

func TestApplyConfigurationPlanAddsServices(t *testing.T) {
	config := Configuration{
		{Package: "samba4", Name: "samba", Type: "samba", Options: map[string]string{"workgroup": "WORKGROUP"}},
		{Package: "system", Name: "ntp", Type: "timeserver", Options: map[string]string{"enabled": "1"}},
		{Package: "system", Name: "system", Type: "system", Options: map[string]string{"hostname": "router"}},
		{Package: "network", Name: "lan", Type: "interface", Options: map[string]string{"ipaddr": "192.168.8.2"}},
	}
	actual := Configuration{
		{Package: "samba4", Name: "samba", Type: "samba", Options: map[string]string{"workgroup": "OLD"}},
		{Package: "system", Name: "ntp", Type: "timeserver", Options: map[string]string{"enabled": "0"}},
		{Package: "system", Name: "system", Type: "system", Options: map[string]string{"hostname": "router"}},
		{Package: "network", Name: "lan", Type: "interface", Options: map[string]string{"ipaddr": "192.168.8.1"}},
	}
	plan := applyConfigurationPlan(reloadPlan{}, config, actual)
	if !plan.Reboot {
		t.Fatal("network change did not request a reboot")
	}
	if len(plan.Services) != 2 || plan.Services[0].Name != "samba4" || plan.Services[1].Name != "chronyd" {
		t.Fatalf("unexpected services: %#v", plan.Services)
	}
	if plan := applyConfigurationPlan(reloadPlan{}, actual, actual); len(plan.Services) != 0 {
		t.Fatalf("unchanged sections activated services: %#v", plan.Services)
	}
}
