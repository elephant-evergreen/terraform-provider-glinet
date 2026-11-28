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
		{Section{Package: "system", Name: "system", Type: "system"}, "system", true},
		{Section{Package: "system", Name: "status", Type: "led"}, "led", true},
		{Section{Package: "system", Name: "ntp", Type: "system"}, "system", true},
		{Section{Package: "qos", Name: "wan", Type: "wan"}, "qos", true},
		{Section{Package: "gl-mesh", Name: "base", Type: "base"}, "gl-mesh", true},
		{Section{Package: "repeater", Name: "main", Type: "main"}, "repeater", true},
		{Section{Package: "gl_s2s", Name: "global", Type: "global"}, "gl_s2s", true},
		{Section{Package: "dropbear", Name: "@dropbear[0]", Type: "dropbear"}, "dropbear", true},
		{Section{Package: "mtkhnat", Name: "global", Type: "global"}, "mtkhnat", true},
		{Section{Package: "wireguard_server", Name: "servers", Type: "servers"}, "wireguard_server", true},
		{Section{Package: "network", Name: "lan"}, "", false},
		{Section{Package: "kmwan", Name: "global"}, "kmwan", true},
		{Section{Package: "gl_timer", Name: "global"}, "gl_timer", true},
		{Section{Package: "gl-black_white_list", Name: "iot"}, "gl-black_white_list", true},
		{Section{Package: "parental_control_v2", Name: "global"}, "parental_control", true},
		{Section{Package: "route_policy", Name: "global"}, "edgerouter", true},
		{Section{Package: "gl_ddns", Name: "global"}, "gl_ddns", true},
		{Section{Package: "gl_dpi", Name: "gl_dpi"}, "gl_dpi", true},
		{Section{Package: "gl_dpi_content_protection", Name: "dpi"}, "gl_dpi", true},
		{Section{Package: "fullconenat", Name: "global"}, "fullconenat", true},
		{Section{Package: "gl_led", Name: "led"}, "gl_led", true},
		{Section{Package: "uhttpd", Name: "main"}, "uhttpd", true},
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

func TestEveryManagedPackageHasActivationContract(t *testing.T) {
	cloud := map[string]bool{"gl-cloud": true}
	configOnly := map[string]string{
		"wireguard.proxy":  "the stock runtime reads the global proxy flag without an init service",
		"glconfig.service": "the stock UI reads the service mode without an init service",
	}
	for scope := range allowedOptions {
		packageName, kind, _ := strings.Cut(scope, ".")
		if cloud[packageName] {
			continue
		}
		if _, ok := configOnly[scope]; ok {
			continue
		}
		config := Configuration{{Package: packageName, Type: kind, Name: "audit", Options: map[string]string{"hostname": "a"}}}
		actual := Configuration{{Package: packageName, Type: kind, Name: "audit", Options: map[string]string{"hostname": "b"}}}
		plan := applyConfigurationPlan(reloadPlan{}, config, actual)
		if plan.Reboot || plan.Wireless || plan.DNS || plan.DHCP || plan.Firewall || plan.Cloud || len(plan.Services) > 0 {
			continue
		}
		if _, ok := serviceFor(Section{Package: packageName, Type: kind}); ok {
			continue
		}
		t.Errorf("%s has no reload path, service activation, or recorded exemption", scope)
	}
}

func TestDropbearActivationIsDeferredOnlyForEndpointChanges(t *testing.T) {
	stable := Configuration{{Package: "dropbear", Type: "dropbear", Name: "@dropbear[0]", Options: map[string]string{"enable": "1", "Port": "22"}}}
	plan := func() reloadPlan {
		value := reloadPlan{}
		addService(&value, dropbearService)
		return value
	}
	if got := resolveDeferred(plan(), stable, 22); got.Services[0].Deferred {
		t.Fatal("a change that keeps the management endpoint was deferred")
	}
	if got := resolveDeferred(plan(), stable, 2222); !got.Services[0].Deferred {
		t.Fatal("a port change was activated inside the transaction")
	}
	disabled := Configuration{{Package: "dropbear", Type: "dropbear", Name: "@dropbear[0]", Options: map[string]string{"enable": "0", "Port": "22"}}}
	if got := resolveDeferred(plan(), disabled, 22); !got.Services[0].Deferred {
		t.Fatal("disabling SSH was activated inside the transaction")
	}
	if got := resolveDeferred(plan(), stable, 0); !got.Services[0].Deferred {
		t.Fatal("an unknown management port was activated inside the transaction")
	}
	if strings.Contains(configurationReload(reloadPlan{Services: []serviceAction{{Name: "dropbear", Deferred: true}}}), "dropbear") {
		t.Fatal("a deferred service ran inside the transaction reload")
	}
}
