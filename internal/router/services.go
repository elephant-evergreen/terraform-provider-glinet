package router

import "strconv"

type serviceAction struct {
	Name         string
	Process      string
	EnabledCheck string
	Command      string
	Optional     bool
	Deferred     bool
	Disruptive   bool
}

func (s serviceAction) command() string {
	if s.Command == "" {
		return "restart"
	}
	return s.Command
}

func (s serviceAction) path() string {
	return "/etc/init.d/" + s.Name
}

func (s serviceAction) waitScript(failure string) string {
	script := "if " + s.EnabledCheck + "; then\n"
	script += "for _ in 1 2 3 4 5 6 7 8 9 10; do pidof " + s.Process + " >/dev/null 2>&1 && break; sleep 1; done\n"
	script += "pidof " + s.Process + " >/dev/null 2>&1 || " + failure + "\n"
	script += "fi\n"
	return script
}

func (s serviceAction) script() string {
	body := s.path() + " " + s.command() + " >/dev/null 2>&1 || exit 75\n"
	if s.Process != "" && s.EnabledCheck != "" {
		body += s.waitScript("exit 75")
	}
	if s.Optional {
		return "if [ -x " + s.path() + " ]; then\n" + body + "fi\n"
	}
	return body
}

// deferredScript activates a service after the transaction is confirmed. It
// records the result instead of failing the already committed transaction.
func (s serviceAction) deferredScript(result string) string {
	body := "rc=0\n"
	if s.Optional {
		body += "if [ -x " + s.path() + " ]; then\n"
	}
	body += s.path() + " " + s.command() + " >/dev/null 2>&1 || rc=1\n"
	if s.Process != "" && s.EnabledCheck != "" {
		body += "if [ \"$rc\" = 0 ] && " + s.EnabledCheck + "; then\n"
		body += "for _ in 1 2 3 4 5 6 7 8 9 10; do pidof " + s.Process + " >/dev/null 2>&1 && break; sleep 1; done\n"
		body += "pidof " + s.Process + " >/dev/null 2>&1 || rc=1\n"
		body += "fi\n"
	}
	if s.Optional {
		body += "fi\n"
	}
	return body + "printf '%s' \"$rc\" > " + quote(result) + "\n"
}

var (
	sambaService       = serviceAction{Name: "samba4"}
	openvpnService     = serviceAction{Name: "openvpn", Process: "openvpn", EnabledCheck: `uci show openvpn 2>/dev/null | grep -q "\.enabled='1'"`}
	tailscaleService   = serviceAction{Name: "tailscale"}
	sqmService         = serviceAction{Name: "sqm"}
	adguardService     = serviceAction{Name: "adguardhome", Process: "AdGuardHome", EnabledCheck: `[ "$(uci -q get adguardhome.config.enabled)" = 1 ]`}
	minidlnaService    = serviceAction{Name: "minidlna", Process: "minidlnad", EnabledCheck: `[ "$(uci -q get minidlna.config.enabled)" = 1 ]`}
	upnpService        = serviceAction{Name: "miniupnpd", Process: "miniupnpd", EnabledCheck: `[ "$(uci -q get upnpd.config.enabled)" = 1 ]`}
	zerotierService    = serviceAction{Name: "zerotier", Process: "zerotier-one", EnabledCheck: `[ "$(uci -q get zerotier.gl.enabled)" = 1 ]`}
	torService         = serviceAction{Name: "tor", Process: "tor", EnabledCheck: `[ "$(uci -q get tor.global.enable)" = 1 ]`}
	ntpService         = serviceAction{Name: "chronyd", Process: "chronyd", EnabledCheck: `[ "$(uci -q get system.ntp.enabled)" = 1 ]`}
	kmwanService       = serviceAction{Name: "kmwan"}
	timerService       = serviceAction{Name: "gl_timer"}
	filterService      = serviceAction{Name: "gl-black_white_list"}
	parentalService    = serviceAction{Name: "parental_control"}
	routePolicyService = serviceAction{Name: "edgerouter"}
	ddnsService        = serviceAction{Name: "gl_ddns"}
	dpiService         = serviceAction{Name: "gl_dpi"}
	fullConeService    = serviceAction{Name: "fullconenat"}
	ledService         = serviceAction{Name: "gl_led"}
	uhttpdService      = serviceAction{Name: "uhttpd"}
	qosService         = serviceAction{Name: "qos", Optional: true}
	meshService        = serviceAction{Name: "gl-mesh", Optional: true, Disruptive: true}
	repeaterService    = serviceAction{Name: "repeater", Optional: true, Disruptive: true}
	s2sService         = serviceAction{Name: "gl_s2s", Optional: true, Disruptive: true}
	systemService      = serviceAction{Name: "system", Command: "reload", Optional: true}
	systemLEDService   = serviceAction{Name: "led", Optional: true}
	mtkhnatService     = serviceAction{Name: "mtkhnat", Optional: true}
	wireguardService   = serviceAction{Name: "wireguard_server", Optional: true}
	dropbearService    = serviceAction{Name: "dropbear", Command: "reload", Process: "dropbear", EnabledCheck: `[ "$(uci -q get dropbear.@dropbear[0].enable)" = 1 ]`, Deferred: true, Disruptive: true}
)

func serviceFor(section Section) (serviceAction, bool) {
	switch section.Package {
	case "samba4":
		return sambaService, true
	case "openvpn":
		return openvpnService, true
	case "tailscale":
		return tailscaleService, true
	case "sqm":
		return sqmService, true
	case "adguardhome":
		return adguardService, true
	case "minidlna":
		return minidlnaService, true
	case "upnpd":
		return upnpService, true
	case "zerotier":
		return zerotierService, true
	case "tor":
		return torService, true
	case "system":
		switch section.Type {
		case "timeserver":
			return ntpService, true
		case "system":
			return systemService, true
		case "led":
			return systemLEDService, true
		}
	case "kmwan":
		return kmwanService, true
	case "gl_timer":
		return timerService, true
	case "gl-black_white_list":
		return filterService, true
	case "parental_control_v2":
		return parentalService, true
	case "route_policy", "edgerouter":
		return routePolicyService, true
	case "gl_ddns":
		return ddnsService, true
	case "gl_dpi", "gl_dpi_content_protection":
		return dpiService, true
	case "fullconenat":
		return fullConeService, true
	case "gl_led":
		return ledService, true
	case "uhttpd":
		return uhttpdService, true
	case "qos":
		return qosService, true
	case "gl-mesh":
		return meshService, true
	case "repeater":
		return repeaterService, true
	case "gl_s2s":
		return s2sService, true
	case "dropbear":
		return dropbearService, true
	case "mtkhnat":
		return mtkhnatService, true
	case "wireguard_server":
		return wireguardService, true
	}
	return serviceAction{}, false
}

func addService(plan *reloadPlan, action serviceAction) {
	for _, existing := range plan.Services {
		if existing.Name == action.Name {
			return
		}
	}
	plan.Services = append(plan.Services, action)
}

// resolveDeferred decides whether an activation has to wait for the confirmed
// transaction. The SSH service waits only when activating it would move or
// close the endpoint the provider is connected through.
func resolveDeferred(plan reloadPlan, config Configuration, managementPort int) reloadPlan {
	for i := range plan.Services {
		if plan.Services[i].Name != dropbearService.Name || !plan.Services[i].Deferred {
			continue
		}
		plan.Services[i].Deferred = dropbearEndpointChange(config, managementPort)
	}
	return plan
}

func dropbearEndpointChange(config Configuration, managementPort int) bool {
	for _, section := range config {
		if section.Package != "dropbear" || section.Type != "dropbear" {
			continue
		}
		if section.Options["enable"] == "0" {
			return true
		}
		if managementPort == 0 {
			return true
		}
		return section.Options["Port"] != strconv.Itoa(managementPort)
	}
	return false
}
