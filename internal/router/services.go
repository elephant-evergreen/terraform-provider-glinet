package router

type serviceAction struct {
	Name         string
	Process      string
	EnabledCheck string
}

func (s serviceAction) script() string {
	script := "/etc/init.d/" + s.Name + " restart >/dev/null 2>&1 || exit 75\n"
	if s.Process == "" || s.EnabledCheck == "" {
		return script
	}
	script += "if " + s.EnabledCheck + "; then\n"
	script += "for _ in 1 2 3 4 5 6 7 8 9 10; do pidof " + s.Process + " >/dev/null 2>&1 && break; sleep 1; done\n"
	script += "pidof " + s.Process + " >/dev/null 2>&1 || exit 75\n"
	script += "fi\n"
	return script
}

var (
	sambaService     = serviceAction{Name: "samba4"}
	openvpnService   = serviceAction{Name: "openvpn", Process: "openvpn", EnabledCheck: `uci show openvpn 2>/dev/null | grep -q "\.enabled='1'"`}
	tailscaleService = serviceAction{Name: "tailscale"}
	sqmService       = serviceAction{Name: "sqm"}
	adguardService   = serviceAction{Name: "adguardhome", Process: "AdGuardHome", EnabledCheck: `[ "$(uci -q get adguardhome.config.enabled)" = 1 ]`}
	minidlnaService  = serviceAction{Name: "minidlna", Process: "minidlnad", EnabledCheck: `[ "$(uci -q get minidlna.config.enabled)" = 1 ]`}
	upnpService      = serviceAction{Name: "miniupnpd", Process: "miniupnpd", EnabledCheck: `[ "$(uci -q get upnpd.config.enabled)" = 1 ]`}
	zerotierService  = serviceAction{Name: "zerotier", Process: "zerotier-one", EnabledCheck: `[ "$(uci -q get zerotier.gl.enabled)" = 1 ]`}
	torService       = serviceAction{Name: "tor", Process: "tor", EnabledCheck: `[ "$(uci -q get tor.global.enable)" = 1 ]`}
	ntpService       = serviceAction{Name: "chronyd", Process: "chronyd", EnabledCheck: `[ "$(uci -q get system.ntp.enabled)" = 1 ]`}
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
		if section.Name == "ntp" && section.Type == "timeserver" {
			return ntpService, true
		}
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
