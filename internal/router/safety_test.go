package router

import (
	"context"
	"strings"
	"testing"
)

func bridgeConfig(brLAN, brNAS []string) Configuration {
	return Configuration{
		{Package: "network", Name: "@device[0]", Type: "device", Options: map[string]string{"name": "br-lan", "type": "bridge"}, Lists: map[string][]string{"ports": brLAN}},
		{Package: "network", Name: "home_iac_nas_bridge", Type: "device", Options: map[string]string{"name": "br-vlan11", "type": "bridge"}, Lists: map[string][]string{"ports": brNAS}},
	}
}

func bridgeState() uciState {
	return uciState{"network": {
		{Name: "lan_bridge", Type: "device", Options: map[string]string{"name": "br-lan", "type": "bridge"}, Lists: map[string][]string{"ports": {"lan1", "lan2", "lan3", "lan4", "lan5"}}},
	}}
}

func TestBridgePortsPreserveExistingMembership(t *testing.T) {
	if err := checkBridgePorts(bridgeConfig([]string{"lan1", "lan3", "lan4", "lan5"}, []string{"lan2"}), bridgeState()); err != nil {
		t.Fatal(err)
	}
	for name, config := range map[string]Configuration{
		"removes a port": bridgeConfig([]string{"lan1", "lan3", "lan4"}, []string{"lan2"}),
		"adds unmanaged": bridgeConfig([]string{"lan1", "lan3", "lan4", "lan5", "lan6"}, []string{"lan2"}),
	} {
		t.Run(name, func(t *testing.T) {
			if err := checkBridgePorts(config, bridgeState()); err == nil {
				t.Fatal("Unsafe bridge membership change was accepted")
			}
		})
	}
}

func TestBridgePortsPreserveMatchedBridge(t *testing.T) {
	config := Configuration{
		{Package: "network", Name: "@device[0]", Type: "device", Match: map[string]string{"name": "br-lan"}, Lists: map[string][]string{"ports": {"lan1", "lan3", "lan4", "lan5"}}},
		{Package: "network", Name: "home_iac_nas_bridge", Type: "device", Options: map[string]string{"name": "br-vlan11", "type": "bridge"}, Lists: map[string][]string{"ports": {"lan2"}}},
	}
	state := uciState{"network": {
		{Name: "lan_bridge", Type: "device", Options: map[string]string{"name": "br-lan", "type": "bridge"}, Lists: map[string][]string{"ports": {"lan1", "lan2", "lan3", "lan4", "lan5"}}},
	}}
	if err := checkBridgePorts(config, state); err != nil {
		t.Fatal(err)
	}
	config[0].Lists["ports"] = []string{"lan1", "lan2", "lan3", "lan4", "lan5"}
	if err := checkBridgePorts(config, state); err == nil {
		t.Fatal("Adding the NAS port back to the private bridge was accepted")
	}
}

func TestPrivateLANAddressChangeIsRejected(t *testing.T) {
	config := Configuration{{Package: "network", Name: "lan", Type: "interface", Options: map[string]string{"ipaddr": "192.168.8.1", "netmask": "255.255.255.0"}}}
	state := uciState{"network": {{Name: "lan", Type: "interface", Options: map[string]string{"ipaddr": "192.168.8.1", "netmask": "255.255.255.0"}}}}
	if err := checkPrivateLAN(config, state); err != nil {
		t.Fatal(err)
	}
	state["network"] = []uciSection{{Name: "lan", Type: "interface", Options: map[string]string{"ipaddr": "192.168.8.1", "netmask": "255.255.254.0"}}}
	if err := checkPrivateLAN(config, state); err == nil {
		t.Fatal("Private LAN netmask change was accepted")
	}
}

func topologyConfig() Configuration {
	return Configuration{
		{Package: "network", Name: "vlan11", Type: "interface", Options: map[string]string{"device": "br-vlan11", "ipaddr": "192.168.11.1", "netmask": "255.255.255.0"}},
		{Package: "network", Name: "home_iac_nas_bridge", Type: "device", Options: map[string]string{"name": "br-vlan11", "type": "bridge"}, Lists: map[string][]string{"ports": {"lan2"}}},
		{Package: "wireless", Name: "home_iac_visitor_5g", Type: "wifi-iface", Options: map[string]string{"ifname": "rax4", "network": "vlan12"}},
		{Package: "firewall", Name: "home_iac_lan_zone", Type: "zone", Options: map[string]string{"name": "lan"}},
		{Package: "firewall", Name: "home_iac_guest_zone", Type: "zone", Options: map[string]string{"name": "guest"}},
		{Package: "firewall", Name: "home_iac_iot_zone", Type: "zone", Options: map[string]string{"name": "iot"}},
		{Package: "firewall", Name: "home_iac_nas_zone", Type: "zone", Options: map[string]string{"name": "nas"}},
		{Package: "firewall", Name: "home_iac_wan_zone", Type: "zone", Options: map[string]string{"name": "wan"}},
		{Package: "firewall", Name: "@forwarding[0]", Type: "forwarding", Match: map[string]string{"src": "guest", "dest": "wan"}, Options: map[string]string{"enabled": "1"}},
		{Package: "firewall", Name: "home_iac_nas_access", Type: "rule", Options: map[string]string{"target": "ACCEPT", "src": "lan", "dest": "nas"}},
	}
}

func topologyState() uciState {
	return uciState{
		"network": {
			{Name: "lan", Type: "interface", Options: map[string]string{"device": "br-lan"}},
			{Name: "guest", Type: "interface", Options: map[string]string{"device": "br-guest"}},
			{Name: "vlan11", Type: "interface", Options: map[string]string{"device": "br-vlan11", "home_iac_owner": "1"}},
			{Name: "vlan12", Type: "interface", Options: map[string]string{"device": "br-vlan12", "home_iac_owner": "1"}},
			{Name: "nas_alias", Type: "interface", Options: map[string]string{"device": "@vlan11"}},
		},
		"wireless": {
			{Name: "home_iac_visitor_5g", Type: "wifi-iface", Options: map[string]string{"ifname": "rax4", "network": "vlan12"}},
		},
		"firewall": {
			{Name: "guest", Type: "zone", Options: map[string]string{"name": "guest"}},
			{Name: "wan", Type: "zone", Options: map[string]string{"name": "wan"}},
			{Name: "guest_wan", Type: "forwarding", Options: map[string]string{"src": "guest", "dest": "wan"}},
		},
	}
}

func TestTopologyOwnershipRejectsUnmanagedConflicts(t *testing.T) {
	config := topologyConfig()
	if err := checkTopologyOwnership(config, topologyState()); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(uciState){
		"wifi on managed network": func(state uciState) {
			state["wireless"] = append(state["wireless"], uciSection{Name: "extra", Type: "wifi-iface", Options: map[string]string{"network": "vlan11"}})
		},
		"wifi on managed bridge": func(state uciState) {
			state["wireless"] = append(state["wireless"], uciSection{Name: "extra", Type: "wifi-iface", Options: map[string]string{"network": "nas_alias"}})
		},
		"radio conflict": func(state uciState) {
			state["wireless"] = append(state["wireless"], uciSection{Name: "extra", Type: "wifi-iface", Options: map[string]string{"ifname": "rax4"}})
		},
		"zone on managed network": func(state uciState) {
			state["firewall"] = append(state["firewall"], uciSection{Name: "other", Type: "zone", Options: map[string]string{"name": "other"}, Lists: map[string][]string{"network": {"vlan11"}}})
		},
		"duplicate zone": func(state uciState) {
			state["firewall"] = append(state["firewall"], uciSection{Name: "shadow", Type: "zone", Options: map[string]string{"name": "guest"}})
		},
		"unmanaged forwarding": func(state uciState) {
			state["firewall"] = append(state["firewall"], uciSection{Name: "other", Type: "forwarding", Options: map[string]string{"src": "guest", "dest": "nas"}})
		},
		"unmanaged redirect": func(state uciState) {
			state["firewall"] = append(state["firewall"], uciSection{Name: "redirect", Type: "redirect", Options: map[string]string{"src": "guest", "dest": "nas"}})
		},
		"accept into nas": func(state uciState) {
			state["firewall"] = append(state["firewall"], uciSection{Name: "other", Type: "rule", Options: map[string]string{"target": "ACCEPT", "src": "guest", "dest": "nas"}})
		},
		"accept from wan": func(state uciState) {
			state["firewall"] = append(state["firewall"], uciSection{Name: "other", Type: "rule", Options: map[string]string{"target": "ACCEPT", "src": "wan", "dest": "lan"}})
		},
		"management accept": func(state uciState) {
			state["firewall"] = append(state["firewall"], uciSection{Name: "other", Type: "rule", Options: map[string]string{"target": "ACCEPT", "src": "extra", "proto": "tcp", "dest_port": "22"}})
		},
		"wan accept": func(state uciState) {
			state["firewall"] = append(state["firewall"], uciSection{Name: "other", Type: "rule", Options: map[string]string{"target": "ACCEPT", "src": "lan", "dest": "wan"}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			state := topologyState()
			mutate(state)
			if err := checkTopologyOwnership(config, state); err == nil {
				t.Fatal("Unmanaged conflict was accepted")
			}
		})
	}
}

func TestDHCPReservationsRejectConflicts(t *testing.T) {
	config := Configuration{
		{Package: "network", Name: "lan", Type: "interface", Options: map[string]string{"ipaddr": "192.168.8.1", "netmask": "255.255.255.0"}},
		{Package: "network", Name: "vlan11", Type: "interface", Options: map[string]string{"ipaddr": "192.168.11.1", "netmask": "255.255.255.0"}},
		{Package: "dhcp", Name: "home_iac_nas_dhcp", Type: "dhcp", Options: map[string]string{"interface": "vlan11", "start": "100", "limit": "150"}},
		{Package: "dhcp", Name: "home_iac_nas_lease", Type: "host", Options: map[string]string{"ip": "192.168.11.10", "mac": "02:00:00:00:00:02"}},
	}
	state := uciState{"dhcp": {
		{Name: "home_iac_nas_lease", Type: "host", Options: map[string]string{"ip": "192.168.11.10", "mac": "02:00:00:00:00:02"}},
	}}
	if err := checkDHCPReservations(config, state, ""); err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		config Configuration
		state  uciState
		leases string
	}{
		"inside pool":    {config: withHost(config, "inside", "192.168.11.120", "02:00:00:00:00:03"), state: state},
		"existing lease": {config: config, state: state, leases: "9999999999 02:00:00:00:00:99 192.168.11.10 host *"},
		"existing host":  {config: config, state: uciState{"dhcp": {{Name: "stock", Type: "host", Options: map[string]string{"ip": "192.168.11.10", "mac": "02:00:00:00:00:99"}}}}, leases: ""},
		"duplicate":      {config: withHost(config, "duplicate", "192.168.11.10", "02:00:00:00:00:04"), state: state},
	} {
		t.Run(name, func(t *testing.T) {
			if err := checkDHCPReservations(test.config, test.state, test.leases); err == nil {
				t.Fatal("DHCP conflict was accepted")
			}
		})
	}
}

func withHost(config Configuration, name, address, mac string) Configuration {
	return append(append(Configuration{}, config...), Section{Package: "dhcp", Name: name, Type: "host", Options: map[string]string{"ip": address, "mac": mac}})
}

func TestIPv6AndUPnPPreconditions(t *testing.T) {
	if err := checkIPv6Inactive(safetyEnvironment{IPv6Enabled: "0", IPv6Forwarding: "0"}); err != nil {
		t.Fatal(err)
	}
	for _, environment := range []safetyEnvironment{
		{IPv6Enabled: "1", IPv6Forwarding: "0"},
		{IPv6Enabled: "0", IPv6Forwarding: "0", IPv6Addresses: "inet6 fe80::1/64 scope link"},
		{IPv6Enabled: "0", IPv6Forwarding: "0", IPv6Routes: "default via fe80::1 dev br-lan"},
	} {
		if err := checkIPv6Inactive(environment); err == nil {
			t.Fatal("Active IPv6 was accepted")
		}
	}
	if err := checkUPnPDisabled(safetyEnvironment{UPnPEnabled: "0"}); err != nil {
		t.Fatal(err)
	}
	for _, environment := range []safetyEnvironment{{UPnPEnabled: "1"}, {UPnPEnabled: "0", UPnPPID: "1234"}} {
		if err := checkUPnPDisabled(environment); err == nil {
			t.Fatal("Active UPnP was accepted")
		}
	}
}

func TestDNSProviderAndStockIncludeChecks(t *testing.T) {
	config := Configuration{{Package: "gl-dns-v2", Name: "@dns[0]", Type: "dns", Options: map[string]string{"provider": "quad9", "proto": "doh"}, Lists: map[string][]string{"servers_list": {"Recommended"}}}}
	client := NewClient(runnerFunc(func(_ context.Context, script string) ([]byte, error) {
		return []byte(`{"server_list":[{"name":"Recommended","url_doh":"https://dns.quad9.net/dns-query"}]}`), nil
	}))
	if err := checkDNSProviders(context.Background(), client, config); err != nil {
		t.Fatal(err)
	}
	config[0].Lists["servers_list"] = []string{"Missing"}
	if err := checkDNSProviders(context.Background(), client, config); err == nil {
		t.Fatal("Missing DNS provider server was accepted")
	}
	state := uciState{"firewall": {{Name: "stock", Type: "include", Options: map[string]string{"path": "/etc/firewall.user"}}}}
	includeClient := NewClient(runnerFunc(func(_ context.Context, script string) ([]byte, error) {
		return []byte("a  /etc/firewall.user\na  /rom/etc/firewall.user\n"), nil
	}))
	if err := checkIncludes(context.Background(), includeClient, state); err != nil {
		t.Fatal(err)
	}
	state["firewall"][0].Options["path"] = "/tmp/override.sh"
	if err := checkIncludes(context.Background(), includeClient, state); err == nil {
		t.Fatal("Unknown include was accepted")
	}
}

func TestSafetyStateLuaRedactsWirelessSecrets(t *testing.T) {
	if !strings.Contains(safetyStateLua, "key=='key'") || !strings.Contains(safetyStateLua, "wpa_psk") {
		t.Fatal("Safety state does not redact Wi-Fi credentials")
	}
}
