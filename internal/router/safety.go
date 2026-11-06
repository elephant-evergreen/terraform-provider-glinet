package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// The safety state is read with UCI's own cursor instead of parsing shell
// output. Wi-Fi secrets are removed before the state leaves the router.
const safetyStateLua = `local uci=require('uci').cursor()
local packages={'network','wireless','dhcp','firewall','gl-dns-v2','eth_ports_config_map'}
local function secret(pkg,key)
 if pkg=='wireless' and (key=='key' or key:match('^key%d$') or key=='wpa_psk' or key=='password' or key=='psk') then return true end
 if pkg=='network' and (key=='private_key' or key=='preshared_key') then return true end
 if pkg=='openvpn' and (key=='auth_user_pass' or key=='key' or key=='password') then return true end
 return false
end
local result={}
for _,pkg in ipairs(packages) do
 local sections={}
 uci:foreach(pkg,nil,function(section)
  local item={name=section['.name'],type=section['.type'],options={},lists={}}
  for key,value in pairs(section) do
   if key:sub(1,1)~='.' and not secret(pkg,key) then
    if type(value)=='table' then item.lists[key]=value else item.options[key]=value end
   end
  end
  sections[#sections+1]=item
 end)
 result[pkg]=sections
end
io.write(require('cjson').encode(result))
`

const safetyEnvironmentScript = `set -eu
# GLINET_SAFETY_ENV
printf 'ipv6_enabled='; uci -q get glipv6.globals.enabled || true
printf '\n'
printf 'ipv6_forwarding='; cat /proc/sys/net/ipv6/conf/all/forwarding || true
printf '\n'
printf '%s\n' '@@addresses@@'
ip -6 addr show scope global || true
ip -6 addr show scope link || true
printf '%s\n' '@@routes@@'
ip -6 route show || true
printf '%s\n' '@@upnp@@'
uci -q get upnpd.config.enabled || true
printf '\n'
pidof miniupnpd || true
printf '\n'
printf '%s\n' '@@adguard@@'
uci -q get adguardhome.config.enabled || true
printf '\n'
uci -q get adguardhome.config.dns_enabled || true
printf '\n'
printf '%s\n' '@@leases@@'
cat /tmp/dhcp.leases || true
`

const safetyRuntimeScript = `set -eu
# GLINET_SAFETY_RUNTIME
printf '%s\n' '@@generated@@'
fw3 -4 -q print
printf '%s\n' '@@running@@'
iptables-save
`

type uciSection struct {
	Name    string              `json:"name"`
	Type    string              `json:"type"`
	Options map[string]string   `json:"options"`
	Lists   map[string][]string `json:"lists"`
}

type uciState map[string][]uciSection

type safetyEnvironment struct {
	IPv6Enabled    string
	IPv6Forwarding string
	IPv6Addresses  string
	IPv6Routes     string
	UPnPEnabled    string
	UPnPPID        string
	AdGuardEnabled string
	AdGuardDNS     string
	Leases         string
}

func (c *Client) readSafetyState(ctx context.Context) (uciState, error) {
	output, err := c.runner.Run(ctx, preflight+"lua - <<'GLINET_SAFETY_STATE'\n"+safetyStateLua+"\nGLINET_SAFETY_STATE\n")
	if err != nil {
		return nil, operationError(err)
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(output, &raw) != nil {
		return nil, errors.New("The router returned invalid configuration state.")
	}
	state := uciState{}
	for packageName, data := range raw {
		if strings.TrimSpace(string(data)) == "{}" {
			state[packageName] = nil
			continue
		}
		var sections []uciSection
		if json.Unmarshal(data, &sections) != nil {
			return nil, errors.New("The router returned invalid configuration sections.")
		}
		for index := range sections {
			if len(sections[index].Options) == 0 {
				sections[index].Options = nil
			}
			if len(sections[index].Lists) == 0 {
				sections[index].Lists = nil
			}
		}
		state[packageName] = sections
	}
	return state, nil
}

func (c *Client) readSafetyEnvironment(ctx context.Context) (safetyEnvironment, error) {
	output, err := c.runner.Run(ctx, safetyEnvironmentScript)
	if err != nil {
		return safetyEnvironment{}, operationError(err)
	}
	text := string(output)
	environment := safetyEnvironment{}
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "ipv6_enabled="):
			environment.IPv6Enabled = strings.TrimPrefix(line, "ipv6_enabled=")
		case strings.HasPrefix(line, "ipv6_forwarding="):
			environment.IPv6Forwarding = strings.TrimPrefix(line, "ipv6_forwarding=")
		}
	}
	environment.IPv6Addresses = between(text, "@@addresses@@", "@@routes@@")
	environment.IPv6Routes = between(text, "@@routes@@", "@@upnp@@")
	upnp := strings.Fields(between(text, "@@upnp@@", "@@adguard@@"))
	if len(upnp) > 0 {
		environment.UPnPEnabled = upnp[0]
	}
	if len(upnp) > 1 {
		environment.UPnPPID = strings.Join(upnp[1:], " ")
	}
	adguard := strings.Fields(between(text, "@@adguard@@", "@@leases@@"))
	if len(adguard) > 0 {
		environment.AdGuardEnabled = adguard[0]
	}
	if len(adguard) > 1 {
		environment.AdGuardDNS = adguard[1]
	}
	environment.Leases = after(text, "@@leases@@")
	return environment, nil
}

func between(source, start, end string) string {
	startIndex := strings.Index(source, start)
	if startIndex < 0 {
		return ""
	}
	remainder := source[startIndex+len(start):]
	endIndex := strings.Index(remainder, end)
	if endIndex < 0 {
		return strings.TrimSpace(remainder)
	}
	return strings.TrimSpace(remainder[:endIndex])
}

func after(source, marker string) string {
	index := strings.Index(source, marker)
	if index < 0 {
		return ""
	}
	return strings.TrimSpace(source[index+len(marker):])
}

func (c *Client) checkConfigurationSafety(ctx context.Context, config Configuration) error {
	needsState := hasPackage(config, "network") || hasPackage(config, "wireless") || hasPackage(config, "firewall") || hasPackage(config, "dhcp") || hasPackage(config, "eth_ports_config_map")
	var state uciState
	if needsState {
		var err error
		state, err = c.readSafetyState(ctx)
		if err != nil {
			return err
		}
	}
	if hasNetworkDevice(config) {
		if err := checkBridgePorts(config, state); err != nil {
			return err
		}
	}
	if hasPackage(config, "network") || hasPackage(config, "wireless") || hasPackage(config, "firewall") || hasPackage(config, "dhcp") {
		if err := checkTopologyOwnership(config, state); err != nil {
			return err
		}
	}
	if hasPackage(config, "network") {
		if err := checkPrivateLAN(config, state); err != nil {
			return err
		}
	}
	needsEnvironment := hasPackage(config, "network") || hasPackage(config, "firewall") || hasDHCPHost(config) || hasPackage(config, "gl-dns-v2")
	var environment safetyEnvironment
	if needsEnvironment {
		var err error
		environment, err = c.readSafetyEnvironment(ctx)
		if err != nil {
			return err
		}
	}
	if hasPackage(config, "network") || hasPackage(config, "firewall") {
		if err := checkIPv6Inactive(environment); err != nil {
			return err
		}
	}
	if hasPackage(config, "firewall") {
		if err := checkUPnPDisabled(environment); err != nil {
			return err
		}
		if err := checkIncludes(ctx, c, state); err != nil {
			return err
		}
	}
	if hasPackage(config, "dhcp") {
		if err := checkDHCPReservations(config, state, environment.Leases); err != nil {
			return err
		}
	}
	if hasPackage(config, "gl-dns-v2") || hasDHCPDNS(config) {
		if environment.AdGuardEnabled == "1" && environment.AdGuardDNS == "1" {
			return errors.New("AdGuard Home owns DNS. Resolve DNS ownership outside Terraform before applying.")
		}
	}
	if hasPackage(config, "gl-dns-v2") {
		if err := checkDNSProviders(ctx, c, config); err != nil {
			return err
		}
	}
	return nil
}

func hasPackage(config Configuration, packageName string) bool {
	for _, section := range config {
		if section.Package == packageName && !section.Absent {
			return true
		}
	}
	return false
}

func hasDHCPDNS(config Configuration) bool {
	for _, section := range config {
		if section.Package == "dhcp" && section.Type == "dnsmasq" && !section.Absent {
			return true
		}
	}
	return false
}

func hasDHCPHost(config Configuration) bool {
	for _, section := range config {
		if section.Package == "dhcp" && section.Type == "host" && !section.Absent {
			return true
		}
	}
	return false
}

func hasNetworkDevice(config Configuration) bool {
	for _, section := range config {
		if section.Package == "network" && section.Type == "device" && !section.Absent {
			return true
		}
	}
	return false
}

func stateSection(state uciState, packageName, name string) (uciSection, bool) {
	for _, section := range state[packageName] {
		if section.Name == name {
			return section, true
		}
	}
	return uciSection{}, false
}

func sectionValues(section uciSection, key string) []string {
	if values, ok := section.Lists[key]; ok {
		return values
	}
	if value := section.Options[key]; value != "" {
		return strings.Fields(value)
	}
	return nil
}

func stateInterfaceDevice(state uciState, name string) string {
	section, ok := stateSection(state, "network", name)
	if !ok {
		return ""
	}
	return section.Options["device"]
}

func resolveNetworkDevices(state uciState, network string) map[string]bool {
	pending := []string{network}
	visited := map[string]bool{}
	devices := map[string]bool{}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if visited[current] {
			continue
		}
		visited[current] = true
		section, ok := stateSection(state, "network", current)
		if !ok {
			continue
		}
		for _, key := range []string{"device", "ifname"} {
			for _, value := range sectionValues(section, key) {
				if strings.HasPrefix(value, "@") {
					pending = append(pending, value[1:])
				} else {
					devices[value] = true
				}
			}
		}
	}
	return devices
}

func managedSectionName(config Configuration, packageName, sectionType, name string) bool {
	for _, section := range config {
		if section.Package == packageName && section.Type == sectionType && !section.Absent && section.Name == name {
			return true
		}
	}
	return false
}

func checkBridgePorts(config Configuration, state uciState) error {
	desired := map[string]map[string]bool{}
	for _, section := range config {
		if section.Package != "network" || section.Type != "device" || section.Absent {
			continue
		}
		name := section.Options["name"]
		if name == "" {
			name = section.Match["name"]
		}
		if name == "" || (section.Options["type"] != "bridge" && len(section.Lists["ports"]) == 0) {
			continue
		}
		desired[name] = stringSet(section.Lists["ports"])
	}
	if len(desired) == 0 {
		return nil
	}
	current := map[string]map[string]bool{}
	for _, section := range state["network"] {
		if section.Type == "device" && section.Options["type"] == "bridge" && section.Options["name"] != "" {
			current[section.Options["name"]] = stringSet(section.Lists["ports"])
		}
	}
	type delta struct{ added, removed map[string]bool }
	deltas := map[string]delta{}
	for name, ports := range desired {
		deltas[name] = delta{added: difference(ports, current[name]), removed: difference(current[name], ports)}
	}
	for name, change := range deltas {
		for port := range change.added {
			moved := false
			for other, otherChange := range deltas {
				if other != name && otherChange.removed[port] {
					moved = true
				}
			}
			if !moved {
				return fmt.Errorf("bridge %s would add port %s without removing it from another managed bridge. Declare the existing membership before applying.", name, port)
			}
		}
		for port := range change.removed {
			moved := false
			for other, otherChange := range deltas {
				if other != name && otherChange.added[port] {
					moved = true
				}
			}
			if !moved {
				return fmt.Errorf("bridge %s would remove port %s without assigning it to another managed bridge. Preserve the existing port assignment or manage the move explicitly.", name, port)
			}
		}
	}
	return nil
}

func checkPrivateLAN(config Configuration, state uciState) error {
	for _, section := range config {
		if section.Package != "network" || section.Type != "interface" || section.Name != "lan" || section.Absent {
			continue
		}
		current, ok := stateSection(state, "network", "lan")
		if !ok {
			if section.Options["ipaddr"] != "" || section.Options["netmask"] != "" {
				return errors.New("The private LAN interface is missing on the router. Refusing to create it without an explicit management path.")
			}
			continue
		}
		for _, key := range []string{"ipaddr", "netmask"} {
			if want := section.Options[key]; want != "" && current.Options[key] != want {
				return fmt.Errorf("Changing the private LAN %s is not supported by this coordinated configuration. Move the management address explicitly before applying.", key)
			}
		}
	}
	return nil
}

func checkTopologyOwnership(config Configuration, state uciState) error {
	managedInterfaces := map[string]bool{}
	for _, section := range config {
		if section.Package == "network" && section.Type == "interface" && !section.Absent {
			managedInterfaces[section.Name] = true
		}
	}
	managedBridges := map[string]bool{}
	for _, section := range config {
		if section.Package == "network" && section.Type == "device" && !section.Absent && section.Options["type"] == "bridge" && section.Options["name"] != "" {
			managedBridges[section.Options["name"]] = true
		}
	}
	for name := range managedInterfaces {
		if device := stateInterfaceDevice(state, name); device != "" {
			managedBridges[device] = true
		}
		managedBridges["br-"+name] = true
	}
	managedZones := map[string]bool{}
	for _, section := range config {
		if section.Package == "firewall" && section.Type == "zone" && !section.Absent && section.Options["name"] != "" {
			managedZones[section.Options["name"]] = true
		}
	}
	managedIfnames := map[string]bool{}
	for _, section := range config {
		if section.Package == "wireless" && section.Type == "wifi-iface" && !section.Absent {
			if ifname := section.Options["ifname"]; ifname != "" {
				managedIfnames[ifname] = true
			}
		}
	}
	for _, section := range state["wireless"] {
		if section.Type == "wifi-iface" && managedWirelessSection(config, section) && section.Options["ifname"] != "" {
			managedIfnames[section.Options["ifname"]] = true
		}
	}
	for _, section := range state["wireless"] {
		if section.Type != "wifi-iface" || managedWirelessSection(config, section) || wirelessDisabled(section) {
			continue
		}
		if ifname := section.Options["ifname"]; ifname != "" && managedIfnames[ifname] {
			return fmt.Errorf("Unmanaged Wi-Fi section %s uses a radio interface owned by the coordinated configuration.", section.Name)
		}
		for _, network := range sectionValues(section, "network") {
			if managedInterfaces[network] {
				return fmt.Errorf("Unmanaged Wi-Fi section %s joins managed network %s. Disable or remove it before applying.", section.Name, network)
			}
			for device := range resolveNetworkDevices(state, network) {
				if managedBridges[device] {
					return fmt.Errorf("Unmanaged Wi-Fi section %s joins managed bridge %s. Disable or remove it before applying.", section.Name, device)
				}
			}
		}
	}
	policyZones := map[string]bool{}
	for name := range managedZones {
		if name != "wan" {
			policyZones[name] = true
		}
	}
	zoneNames := map[string]int{}
	for _, section := range state["firewall"] {
		if section.Type != "zone" || firewallDisabled(section) || section.Options["family"] == "ipv6" {
			continue
		}
		name := section.Options["name"]
		zoneNames[name]++
		if zoneNames[name] > 1 {
			return fmt.Errorf("Multiple firewall zones use the name %s. Resolve the duplicate before applying.", name)
		}
		managed := managedZones[name] || managedFirewallSection(config, section)
		for _, key := range []string{"device", "subnet", "extra_src", "extra_dest"} {
			if section.Options[key] != "" {
				return fmt.Errorf("Firewall zone %s sets %s and could reclassify managed networks. Resolve it before applying.", name, key)
			}
		}
		if managed {
			continue
		}
		for _, network := range sectionValues(section, "network") {
			if managedInterfaces[network] {
				return fmt.Errorf("Unmanaged firewall zone %s classifies managed network %s. Resolve the zone before applying.", name, network)
			}
			for device := range resolveNetworkDevices(state, network) {
				if managedBridges[device] {
					return fmt.Errorf("Unmanaged firewall zone %s classifies managed bridge %s. Resolve the zone before applying.", name, device)
				}
			}
		}
	}
	for _, section := range state["firewall"] {
		if firewallDisabled(section) || section.Options["family"] == "ipv6" {
			continue
		}
		switch section.Type {
		case "redirect":
			if managedFirewallSection(config, section) {
				continue
			}
			return errors.New("An unmanaged enabled port redirect exists. Resolve it before applying.")
		case "forwarding":
			if managedFirewallSection(config, section) {
				continue
			}
			source, destination := section.Options["src"], section.Options["dest"]
			if source == "*" || destination == "*" || policyZones[destination] || source == "nas" {
				return fmt.Errorf("Unmanaged firewall forwarding %s is enabled. Resolve it before applying.", section.Name)
			}
		case "rule":
			if managedFirewallSection(config, section) || section.Options["target"] != "ACCEPT" {
				continue
			}
			source, destination := section.Options["src"], section.Options["dest"]
			if destination == "wan" {
				return fmt.Errorf("Unmanaged ACCEPT rule %s can bypass the WAN private-subnet policy. Resolve it before applying.", ruleName(section))
			}
			if source == "*" || destination == "*" || source == "nas" || destination == "nas" {
				return fmt.Errorf("Unmanaged ACCEPT rule %s can bypass a managed network policy. Resolve it before applying.", ruleName(section))
			}
			if policyZones[destination] {
				return fmt.Errorf("Unmanaged ACCEPT rule %s can bypass a managed network policy. Resolve it before applying.", ruleName(section))
			}
			if source != "" && source != "lan" && (destination == "" || destination == "*") && permitsManagementTCP(section) {
				return fmt.Errorf("Unmanaged ACCEPT rule %s can expose a router management port. Resolve it before applying.", ruleName(section))
			}
		}
	}
	return nil
}

func managedWirelessSection(config Configuration, current uciSection) bool {
	for _, section := range config {
		if section.Package != "wireless" || section.Type != current.Type || section.Absent {
			continue
		}
		if section.Name == current.Name {
			return true
		}
		if strings.HasPrefix(section.Name, "@") && len(section.Match) > 0 {
			matched := true
			for key, value := range section.Match {
				if current.Options[key] != value {
					matched = false
				}
			}
			if matched {
				return true
			}
		}
	}
	return false
}

func managedFirewallSection(config Configuration, current uciSection) bool {
	for _, section := range config {
		if section.Package != "firewall" || section.Type != current.Type || section.Absent {
			continue
		}
		if section.Name == current.Name {
			return true
		}
		if strings.HasPrefix(section.Name, "@") && len(section.Match) > 0 {
			matched := true
			for key, value := range section.Match {
				if current.Options[key] != value {
					matched = false
				}
			}
			if matched {
				return true
			}
		}
	}
	return false
}

func ruleName(section uciSection) string {
	if section.Options["name"] != "" {
		return section.Options["name"]
	}
	return section.Name
}

func permitsManagementTCP(section uciSection) bool {
	protocol := section.Options["proto"]
	if protocol == "" {
		protocol = "tcp udp"
	}
	nonTCP := true
	for _, value := range strings.Fields(protocol) {
		switch value {
		case "udp", "icmp", "ipv6-icmp", "icmpv6", "igmp", "esp", "ah", "gre", "1", "2", "17", "58":
		default:
			nonTCP = false
		}
	}
	if nonTCP {
		return false
	}
	ports := section.Options["dest_port"]
	if ports == "" {
		return true
	}
	for _, token := range strings.Fields(ports) {
		match := regexp.MustCompile(`^(\d+)(?:[:-](\d+))?$`).FindStringSubmatch(token)
		if match == nil {
			return true
		}
		low, _ := strconv.Atoi(match[1])
		high := low
		if match[2] != "" {
			high, _ = strconv.Atoi(match[2])
		}
		if low > high {
			return true
		}
		for _, port := range []int{22, 80, 443, 8080, 8443} {
			if low <= port && port <= high {
				return true
			}
		}
	}
	return false
}

func checkIPv6Inactive(environment safetyEnvironment) error {
	if environment.IPv6Enabled != "0" {
		return errors.New("IPv6 is enabled. This configuration only verifies IPv4 isolation; disable IPv6 before applying.")
	}
	if environment.IPv6Forwarding != "0" && environment.IPv6Forwarding != "1" {
		return errors.New("Cannot determine the IPv6 forwarding state. Disable IPv6 before applying.")
	}
	if strings.Contains(environment.IPv6Addresses, "inet6 ") {
		return errors.New("IPv6 addresses are active. This configuration only verifies IPv4 isolation; disable IPv6 before applying.")
	}
	for _, line := range strings.Split(environment.IPv6Routes, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "unreachable ") || strings.HasPrefix(line, "blackhole ") || strings.HasPrefix(line, "prohibit ") {
			continue
		}
		return errors.New("IPv6 routes are active. This configuration only verifies IPv4 isolation; disable IPv6 before applying.")
	}
	return nil
}

func checkUPnPDisabled(environment safetyEnvironment) error {
	if environment.UPnPEnabled != "0" || strings.TrimSpace(environment.UPnPPID) != "" {
		return errors.New("UPnP and NAT-PMP must be disabled and miniupnpd must be stopped; they can add firewall rules outside Terraform.")
	}
	return nil
}

var stockFirewallIncludes = map[string]bool{
	"/etc/firewall.user":                    true,
	"/etc/firewall.nat6":                    true,
	"/etc/firewall.dns_order":               true,
	"/etc/netifyd/iptables-init.sh":         true,
	"/usr/bin/rtp2.sh":                      true,
	"/etc/firewall.security":                true,
	"/etc/firewall.pf":                      true,
	"/usr/lib/gl-upnp/firewall-reload.sh":   true,
	"/etc/firewall.tor.fw3":                 true,
	"/usr/share/miniupnpd/firewall.include": true,
	"/etc/gl_dpi_qos_firewall_reload.sh":    true,
	"/usr/bin/gl_block.sh":                  true,
}

func checkIncludes(ctx context.Context, client *Client, state uciState) error {
	for _, section := range state["firewall"] {
		if section.Type != "include" || firewallDisabled(section) {
			continue
		}
		path := section.Options["path"]
		if path == "" || !stockFirewallIncludes[path] || (section.Options["type"] != "" && section.Options["type"] != "script") {
			return fmt.Errorf("Unsupported firewall include %s. Only stock scripts are accepted.", path)
		}
		output, err := client.runner.Run(ctx, "sha256sum "+quote(path)+" "+quote("/rom"+path))
		if err != nil {
			return fmt.Errorf("Cannot read firewall include %s. Remove it or restore the stock script before applying.", path)
		}
		fields := strings.Fields(string(output))
		if len(fields) < 4 || fields[0] == "" || fields[0] != fields[2] {
			return fmt.Errorf("Firewall include %s does not match the stock ROM script. Restore it before applying.", path)
		}
	}
	return nil
}

func checkDNSProviders(ctx context.Context, client *Client, config Configuration) error {
	for _, section := range config {
		if section.Package != "gl-dns-v2" || section.Type != "dns" || section.Absent {
			continue
		}
		providerName := section.Options["provider"]
		if providerName == "" || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(providerName) {
			return errors.New("The stock DNS provider name is invalid.")
		}
		output, err := client.runner.Run(ctx, "cat "+quote("/etc/dnsproxy/provider/"+providerName))
		if err != nil {
			return fmt.Errorf("Cannot read the stock DNS provider %s.", providerName)
		}
		var metadata struct {
			Servers []struct {
				Name   string `json:"name"`
				URLDoh string `json:"url_doh"`
				URLDot string `json:"url_dot"`
				URLUDP string `json:"url_udp"`
			} `json:"server_list"`
		}
		if json.Unmarshal(output, &metadata) != nil {
			return fmt.Errorf("The stock DNS provider %s returned invalid metadata.", providerName)
		}
		protocol := section.Options["proto"]
		servers := section.Lists["servers_list"]
		if len(servers) == 0 {
			return errors.New("The coordinated DNS setting must name at least one stock DNS server.")
		}
		for _, wanted := range servers {
			found := false
			for _, server := range metadata.Servers {
				if server.Name != wanted {
					continue
				}
				found = true
				var endpoint string
				switch protocol {
				case "doh":
					endpoint = server.URLDoh
				case "dot":
					endpoint = server.URLDot
				case "udp":
					endpoint = server.URLUDP
				}
				if endpoint == "" {
					return fmt.Errorf("The stock DNS provider %s does not offer %s for %s.", providerName, protocol, wanted)
				}
			}
			if !found {
				return fmt.Errorf("The stock DNS provider %s does not contain %s.", providerName, wanted)
			}
		}
	}
	return nil
}

func (c *Client) verifyRuntimeFirewall(ctx context.Context) error {
	output, err := c.runner.Run(ctx, safetyRuntimeScript)
	if err != nil {
		return operationError(err)
	}
	text := string(output)
	generated := between(text, "@@generated@@", "@@running@@")
	running := after(text, "@@running@@")
	if generated == "" || running == "" {
		return errors.New("The router returned incomplete runtime firewall state.")
	}
	if err := verifyRuntimeFirewall(generated, running); err != nil {
		return fmt.Errorf("The running firewall differs from the generated policy: %w", err)
	}
	return nil
}

func wirelessDisabled(section uciSection) bool {
	switch strings.ToLower(section.Options["disabled"]) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func firewallDisabled(section uciSection) bool {
	switch strings.ToLower(section.Options["enabled"]) {
	case "0", "false", "no", "off":
		return true
	}
	return false
}

func stringSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func difference(a, b map[string]bool) map[string]bool {
	result := map[string]bool{}
	for value := range a {
		if !b[value] {
			result[value] = true
		}
	}
	return result
}

func interfacePrefix(address, netmask string) (netip.Prefix, bool) {
	ip := net.ParseIP(address)
	mask := net.ParseIP(netmask)
	if ip == nil || mask == nil || ip.To4() == nil || mask.To4() == nil {
		return netip.Prefix{}, false
	}
	ones, bits := net.IPMask(mask.To4()).Size()
	if bits != 32 {
		return netip.Prefix{}, false
	}
	parsed, err := netip.ParsePrefix(ip.String() + "/" + strconv.Itoa(ones))
	if err != nil {
		return netip.Prefix{}, false
	}
	return parsed.Masked(), true
}

func prefixBroadcast(prefix netip.Prefix) netip.Addr {
	address := prefix.Addr().As4()
	bits := prefix.Bits()
	value := uint32(address[0])<<24 | uint32(address[1])<<16 | uint32(address[2])<<8 | uint32(address[3])
	hostBits := 32 - bits
	if hostBits == 32 {
		return prefix.Addr()
	}
	value |= (uint32(1) << hostBits) - 1
	return netip.AddrFrom4([4]byte{byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)})
}

func checkDHCPReservations(config Configuration, state uciState, leases string) error {
	interfaces := map[string]netip.Prefix{}
	for _, section := range config {
		if section.Package != "network" || section.Type != "interface" || section.Absent {
			continue
		}
		if prefix, ok := interfacePrefix(section.Options["ipaddr"], section.Options["netmask"]); ok {
			interfaces[section.Name] = prefix
		}
	}
	for _, section := range state["network"] {
		if section.Type != "interface" {
			continue
		}
		if _, exists := interfaces[section.Name]; exists {
			continue
		}
		if prefix, ok := interfacePrefix(section.Options["ipaddr"], section.Options["netmask"]); ok {
			interfaces[section.Name] = prefix
		}
	}
	type pool struct {
		start int
		limit int
	}
	pools := map[string]pool{}
	for _, section := range config {
		if section.Package != "dhcp" || section.Type != "dhcp" || section.Absent {
			continue
		}
		name := section.Options["interface"]
		if name == "" {
			continue
		}
		start, startErr := strconv.Atoi(section.Options["start"])
		limit, limitErr := strconv.Atoi(section.Options["limit"])
		if startErr != nil || limitErr != nil || start < 1 || limit < 1 || start+limit > 255 {
			return fmt.Errorf("DHCP pool for %s is invalid.", name)
		}
		if _, exists := pools[name]; exists {
			return fmt.Errorf("Multiple DHCP pools claim interface %s.", name)
		}
		pools[name] = pool{start: start, limit: limit}
	}
	for _, section := range state["dhcp"] {
		if section.Type != "dhcp" || managedSectionName(config, "dhcp", "dhcp", section.Name) {
			continue
		}
		name := section.Options["interface"]
		if name == "" {
			continue
		}
		if _, exists := pools[name]; exists {
			continue
		}
		start, startErr := strconv.Atoi(section.Options["start"])
		limit, limitErr := strconv.Atoi(section.Options["limit"])
		if startErr == nil && limitErr == nil && start >= 1 && limit >= 1 && start+limit <= 255 {
			pools[name] = pool{start: start, limit: limit}
		}
	}
	type reservation struct {
		name string
		ip   netip.Addr
		mac  string
	}
	reservations := map[string]reservation{}
	for _, section := range config {
		if section.Package != "dhcp" || section.Type != "host" || section.Absent {
			continue
		}
		ipText, mac := section.Options["ip"], strings.ToLower(section.Options["mac"])
		if ipText == "" || mac == "" {
			continue
		}
		ip, err := netip.ParseAddr(ipText)
		if err != nil || !ip.Is4() || ip.IsMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("DHCP reservation %s has an invalid IPv4 address.", section.Name)
		}
		hardware, err := net.ParseMAC(mac)
		if err != nil || len(hardware) != 6 || hardware[0]&1 == 1 {
			return fmt.Errorf("DHCP reservation %s has an invalid unicast MAC address.", section.Name)
		}
		interfaceName := ""
		for name, prefix := range interfaces {
			if prefix.Contains(ip) {
				interfaceName = name
				break
			}
		}
		if interfaceName == "" {
			return fmt.Errorf("DHCP reservation %s is outside every declared network.", section.Name)
		}
		prefix := interfaces[interfaceName]
		if ip == prefix.Addr() || ip == prefixBroadcast(prefix) {
			return fmt.Errorf("DHCP reservation %s uses a network or broadcast address.", section.Name)
		}
		for name, existing := range reservations {
			if existing.ip == ip || existing.mac == mac {
				return fmt.Errorf("DHCP reservations %s and %s reuse the same address or MAC.", name, section.Name)
			}
		}
		reservations[section.Name] = reservation{name: section.Name, ip: ip, mac: mac}
	}
	for _, section := range state["dhcp"] {
		if section.Type != "host" || managedSectionName(config, "dhcp", "host", section.Name) {
			continue
		}
		ip, ipErr := netip.ParseAddr(section.Options["ip"])
		mac := strings.ToLower(section.Options["mac"])
		if ipErr != nil {
			continue
		}
		for name, reservation := range reservations {
			if reservation.ip == ip || reservation.mac == mac {
				return fmt.Errorf("DHCP reservation %s conflicts with the existing reservation %s. Resolve it before applying.", name, section.Name)
			}
		}
	}
	for _, line := range strings.Split(leases, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		ip, ipErr := netip.ParseAddr(fields[2])
		mac := strings.ToLower(fields[1])
		if ipErr != nil {
			continue
		}
		for name, reservation := range reservations {
			if (reservation.ip == ip && reservation.mac != mac) || (reservation.mac == mac && reservation.ip != ip) {
				return fmt.Errorf("DHCP reservation %s conflicts with an existing lease. Resolve it before applying.", name)
			}
		}
	}
	return nil
}
