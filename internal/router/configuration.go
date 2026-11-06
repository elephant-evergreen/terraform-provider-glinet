package router

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"
)

type Section struct {
	Package  string              `json:"package"`
	Name     string              `json:"name"`
	Type     string              `json:"type"`
	Match    map[string]string   `json:"match,omitempty"`
	Options  map[string]string   `json:"options,omitempty"`
	Lists    map[string][]string `json:"lists,omitempty"`
	Absent   bool                `json:"absent,omitempty"`
	Missing  bool                `json:"missing,omitempty"`
	Position *int                `json:"position,omitempty"`
}

type Configuration []Section

var selector = regexp.MustCompile(`^@([A-Za-z0-9_-]+)\[[0-9]+\]$`)

func ValidateConfiguration(config Configuration) error {
	if len(config) == 0 || len(config) > 256 {
		return errors.New("Configure between 1 and 256 UCI sections.")
	}
	seen := map[string]bool{}
	positions := map[int]bool{}
	rules, positioned := 0, 0
	for index, s := range config {
		if s.Missing {
			return errors.New("missing is reserved for observed drift.")
		}
		anonymous := selector.FindStringSubmatch(s.Name)
		if allowedOptionsFor(s.Package, s.Type) == nil || (!ValidSection(s.Name) && anonymous == nil) {
			return errors.New("Unsupported UCI package, section name or section type.")
		}
		if len(s.Options) == 0 {
			s.Options = nil
		}
		if len(s.Lists) == 0 {
			s.Lists = nil
		}
		if len(s.Match) == 0 {
			s.Match = nil
		}
		config[index] = s
		key := s.Package + "." + s.Name
		if seen[key] {
			return errors.New("Each UCI section may appear only once.")
		}
		seen[key] = true
		if anonymous == nil && len(s.Match) > 0 {
			return errors.New("Identity matches are supported only for anonymous sections.")
		}
		if anonymous != nil && (anonymous[1] != s.Type || s.Absent) {
			return errors.New("Anonymous selectors must match the section type and cannot be deleted.")
		}
		if s.Position != nil {
			if s.Package != "firewall" || s.Type != "rule" || (*s.Position < 0 || *s.Position > 4095) {
				return errors.New("position must be a firewall rule index between 0 and 4095.")
			}
			if positions[*s.Position] {
				return errors.New("Firewall rule positions must be unique.")
			}
			positions[*s.Position] = true
		}
		if s.Package == "firewall" && s.Type == "rule" && !s.Absent {
			rules++
			if s.Position != nil {
				positioned++
			}
		}
		if s.Absent && (len(s.Options)+len(s.Lists)+len(s.Match) > 0 || s.Position != nil) {
			return errors.New("Absent sections cannot include options, lists or matches.")
		}
		for k, v := range s.Options {
			if s.Package == "wireless" && ((k == "ssid" && !ValidSSID(v)) || ((k == "disabled" || k == "hidden" || k == "isolate") && v != "0" && v != "1")) {
				return errors.New("Wi-Fi SSIDs must contain 1–32 UTF-8 bytes and flags must be 0 or 1.")
			}
			if !allowedOptionsFor(s.Package, s.Type)[k] || (!validOption(s.Package, s.Type, k, v)) {
				return errors.New("Invalid or secret UCI option. Use key_wo for Wi-Fi keys.")
			}
			if _, ok := s.Lists[k]; ok {
				return errors.New("An option cannot be both a scalar and a list.")
			}
		}
		for k, v := range s.Match {
			if value, ok := s.Options[k]; ok && value != v {
				return errors.New("Managed identity options must equal their match values.")
			}
			if _, ok := s.Lists[k]; ok {
				return errors.New("Identity match options cannot be lists.")
			}
			if !validOption(s.Package, s.Type, k, v) {
				return errors.New("Invalid UCI identity match.")
			}
		}
		for k, vs := range s.Lists {
			if s.Package == "wireless" {
				return errors.New("Wi-Fi attributes must use scalar options, not lists.")
			}
			if !allowedOptionsFor(s.Package, s.Type)[k] {
				return errors.New("Unsupported UCI list option.")
			}
			if len(vs) == 0 {
				s.Lists[k] = nil
			}
			for _, v := range vs {
				if !allowedOptionsFor(s.Package, s.Type)[k] || (!validOption(s.Package, s.Type, k, v)) {
					return errors.New("Invalid or secret UCI list option.")
				}
			}
		}
	}
	if positioned > 0 && positioned != rules {
		return errors.New("Set positions for all managed firewall rules or leave all positions unspecified.")
	}
	return nil
}

func allowedOptionsFor(pkg, kind string) map[string]bool {
	if options := allowedOptions[pkg+"."+kind]; options != nil {
		return options
	}
	if pkg == "network" && strings.HasPrefix(kind, "wireguard_") {
		return allowedOptions["network.wireguard"]
	}
	return nil
}

func validOption(pkg, kind, key, value string) bool {
	options := allowedOptionsFor(pkg, kind)
	return options != nil && options[key] && len(value) <= 1024 && !strings.ContainsFunc(value, unicode.IsControl)
}

func luaQuote(value string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\r", "\\r").Replace(value) + "'"
}

func configurationLua(config Configuration, write bool, secrets ...map[string]string) string {
	type wireSection struct {
		Section
		CreateName string `json:"create_name,omitempty"`
	}
	wire := make([]wireSection, len(config))
	for i, section := range config {
		wire[i].Section = section
		if strings.HasPrefix(section.Name, "@") {
			identity, _ := json.Marshal([]any{section.Package, section.Type, section.Match})
			sum := sha256.Sum256(identity)
			wire[i].CreateName = fmt.Sprintf("tf_glinet_%x", sum[:8])
		}
	}
	data, _ := json.Marshal(wire)
	provided := "{}"
	if len(secrets) > 0 && len(secrets[0]) > 0 {
		encoded, _ := json.Marshal(secrets[0])
		provided = string(encoded)
	}
	source := "local json=require('cjson')\nlocal uci=require('uci').cursor()\nlocal writing=" + fmt.Sprint(write) + "\nlocal provided=json.decode(" + luaQuote(provided) + ")\nlocal spec=json.decode(" + luaQuote(string(data)) + ")\n" + `local result={}
local rule_order={}
local rule_slots={}
local resolved={}
local seen={}
local rule_indices={}
local next_index=0
uci:foreach('firewall',nil,function(s) rule_indices[s['.name']]=next_index; next_index=next_index+1 end)
for i,s in ipairs(spec) do
 local name=s.name
 if name:sub(1,1)=='@' then
  local matches={}
  uci:foreach(s.package,s.type,function(v)
   local ok=true
   for k,want in pairs(s.match or {}) do if v[k]~=want then ok=false end end
   if ok then matches[#matches+1]=v['.name'] end
  end)
  if #matches>1 then os.exit(65) end
  if #matches==1 then name=matches[1] else name=s.create_name; if uci:get(s.package,name) then os.exit(65) end end
 end
 local identity=s.package..'.'..name
 if seen[identity] then os.exit(65) end
 seen[identity]=true
 resolved[i]=name
 local kind=uci:get(s.package,name)
 if writing and not kind and not s.absent and s.package=='wireless' then
  local key=provided['wireless.'..s.name..'.key']
  if type(key)~='string' or key=='' then os.exit(65) end
 end
 if s.package=='firewall' then if s.absent and kind then next_index=next_index-1 elseif not s.absent and not kind then next_index=next_index+1 end end
 if kind and kind~=s.type then os.exit(65) end
 for k,want in pairs(s.match or {}) do
  if kind and uci:get(s.package,name,k)~=want then os.exit(65) end
 end
end
if writing then for _,s in ipairs(spec) do if s.position~=nil and s.position>=next_index then os.exit(65) end end end
for i,s in ipairs(spec) do
 local name=resolved[i]
 local kind=uci:get(s.package,name)
 local actual={package=s.package,name=s.name,type=s.type,match=s.match,absent=s.absent,options={},lists={}}
 if s.position~=nil then actual.position=rule_indices[name] or -1 end
`
	if write {
		source += ` if s.absent then
  if kind then assert(uci:delete(s.package,name)) end
 else
  if not kind then assert(uci:set(s.package,name,s.type)); for k,v in pairs(s.match or {}) do assert(uci:set(s.package,name,k,v)) end end
  for k,v in pairs(s.options or {}) do
   if v=='' then if uci:get(s.package,name,k)~=nil then assert(uci:delete(s.package,name,k)) end
   else assert(uci:set(s.package,name,k,v)) end
  end
  for k,v in pairs(s.lists or {}) do
   if v==json.null or #v==0 then if uci:get(s.package,name,k)~=nil then assert(uci:delete(s.package,name,k)) end
   else assert(uci:set(s.package,name,k,v)) end
  end
 end
`
	} else {
		source += ` if s.absent then
  actual.absent=not kind
 else
  if not kind then actual.missing=true end
  for k,v in pairs(s.options or {}) do
   local value=uci:get(s.package,name,k)
   if type(value)~='string' then if s.package=='wireless' and (k=='disabled' or k=='hidden' or k=='isolate') then value='0' else value='' end end
   actual.options[k]=value
  end
  for k,v in pairs(s.lists or {}) do
   local value=uci:get(s.package,name,k)
   if type(value)=='string' then value={value} elseif type(value)~='table' then value=json.null end
   actual.lists[k]=value
  end
 end
 result[i]=actual
 if s.package=='firewall' and s.type=='rule' and not s.absent and s.position==nil then
  rule_order[#rule_order+1]={value=actual,index=rule_indices[name] or 2147483647,slot=i}
  rule_slots[#rule_slots+1]=i
 end
`
	}
	source += "end\n"
	if write {
		if len(secrets) > 0 {
			for key, value := range secrets[0] {
				parts := strings.SplitN(key, ".", 3)
				if len(parts) != 3 {
					continue
				}
				source += "assert(uci:get(" + luaQuote(parts[0]) + "," + luaQuote(parts[1]) + "))\nassert(uci:set(" + luaQuote(parts[0]) + "," + luaQuote(parts[1]) + "," + luaQuote(parts[2]) + "," + luaQuote(value) + "))\n"
			}
		}
		source += "local ordered={}\nlocal count=0\nuci:foreach('firewall',nil,function() count=count+1 end)\nfor i,s in ipairs(spec) do if s.position~=nil and not s.absent then if s.position>=count then os.exit(65) end; ordered[#ordered+1]={name=resolved[i],position=s.position} end end\ntable.sort(ordered,function(a,b) return a.position<b.position end)\nfor _,s in ipairs(ordered) do assert(uci:reorder('firewall',s.name,count-1)) end\nfor _,s in ipairs(ordered) do assert(uci:reorder('firewall',s.name,s.position)) end\n"
		source += "local touched={}\nfor _,s in ipairs(spec) do touched[s.package]=true end\nfor p in pairs(touched) do assert(uci:save(p)) end\n"
	} else {
		source += "table.sort(rule_order,function(a,b) if a.index==b.index then return a.slot<b.slot end; return a.index<b.index end)\nfor i,slot in ipairs(rule_slots) do result[slot]=rule_order[i].value end\nio.write(json.encode(result))\n"
	}
	return "lua - <<'GLINET_CONFIGURATION_LUA'\n" + source + "GLINET_CONFIGURATION_LUA\n"
}

func (c *Client) ReadConfiguration(ctx context.Context, config Configuration) (Configuration, error) {
	output, err := c.runner.Run(ctx, preflight+configurationLua(config, false, nil))
	if err != nil {
		return nil, operationError(err)
	}
	var actual Configuration
	if json.Unmarshal(output, &actual) != nil || len(actual) != len(config) {
		return nil, errors.New("The router returned invalid configuration data.")
	}
	for i := range actual {
		if len(actual[i].Options) == 0 {
			actual[i].Options = nil
		}
		if len(actual[i].Lists) == 0 {
			actual[i].Lists = nil
		}
		if config[i].Absent && actual[i].Absent {
			actual[i] = config[i]
		}
	}
	return actual, nil
}

func SameConfiguration(a, b Configuration) bool { return reflect.DeepEqual(a, b) }

var secretOptions = map[string]map[string]bool{
	"network.device":      nil,
	"network.interface":   {"private_key": true},
	"wireless.wifi-iface": {"key": true, "key1": true, "key2": true, "key3": true, "wpa_psk": true, "password": true, "psk": true},
	"network.wireguard":   {"preshared_key": true},
	"openvpn.openvpn":     {"auth_user_pass": true, "key": true},
}

func ValidSecretOptions(config Configuration, secrets map[string]string) bool {
	for key, value := range secrets {
		parts := strings.SplitN(key, ".", 3)
		if len(parts) != 3 {
			return false
		}
		packageName, sectionName, optionName := parts[0], parts[1], parts[2]
		if value == "" || len(value) > 4096 || strings.ContainsFunc(value, func(r rune) bool { return r < 32 || r == 127 }) {
			return false
		}
		found := false
		for _, section := range config {
			if section.Package == packageName && section.Name == sectionName && !section.Absent {
				found = true
				break
			}
		}
		if !found {
			return false
		}
		allowed := false
		for _, section := range config {
			if section.Package != packageName || section.Name != sectionName {
				continue
			}
			options := secretOptions[packageName+"."+section.Type]
			if options == nil && packageName == "network" && strings.HasPrefix(section.Type, "wireguard_") {
				options = secretOptions["network.wireguard"]
			}
			if options[optionName] {
				allowed = true
			}
		}
		if !allowed {
			return false
		}
	}
	return true
}

func (c *Client) writeConfiguration(ctx context.Context, config Configuration, secrets map[string]string, safetyConfig Configuration) error {
	if !ValidSecretOptions(config, secrets) {
		return errors.New("Secret options must reference managed sections and supported secret names.")
	}
	actual, secretsMatch, err := c.stableConfiguration(ctx, config, secrets)
	if err != nil {
		return err
	}
	if err := c.checkConfigurationSafety(ctx, safetyConfig); err != nil {
		return err
	}
	if secretsMatch {
		secrets = nil
	}
	for i, s := range config {
		if s.Package == "wireless" && !s.Absent && actual[i].Missing && secrets["wireless."+s.Name+".key"] == "" {
			return errors.New("A missing Wi-Fi section requires its write-only key and a new key generation before it can be restored.")
		}
		if s.Package == "firewall" && s.Type == "rule" && s.Position == nil && s.Name != actual[i].Name {
			return errors.New("Changing managed firewall rule order requires explicit positions for all managed rules.")
		}
	}
	if SameConfiguration(actual, config) && len(secrets) == 0 {
		return nil
	}
	tokenBytes := make([]byte, 16)
	if _, err = rand.Read(tokenBytes); err != nil {
		return errors.New("Cannot initialize a configuration transaction.")
	}
	token := hex.EncodeToString(tokenBytes)
	plan := reloadPlan{}
	for i, s := range config {
		if reflect.DeepEqual(s, actual[i]) || sameSectionExceptManagedMarker(s, actual[i]) {
			continue
		}
		switch s.Package {
		case "network", "eth_ports_config_map":
			plan.Reboot = true
		case "wireless":
			plan.Wireless = true
			if s.Absent || actual[i].Missing {
				plan.Reboot = true
			}
			for _, key := range []string{"device", "network", "ifname", "mode"} {
				if s.Options[key] != actual[i].Options[key] {
					plan.Reboot = true
				}
			}
		case "gl-dns-v2":
			plan.DNS = true
		case "dhcp":
			plan.DHCP = true
		case "firewall":
			plan.Firewall = true
		}
	}
	for key := range secrets {
		switch {
		case strings.HasPrefix(key, "wireless."):
			plan.Wireless = true
		case strings.HasPrefix(key, "network."):
			plan.Reboot = true
		}
	}
	if c.protection == nil {
		return errors.New("Router changes require encrypted backup and management-path protection.")
	}
	if err := c.protection.Protect(ctx, c.runner, plan.Reboot || plan.Wireless); err != nil {
		return err
	}
	_, err = c.runner.Run(ctx, configurationApplyScript(config, token, plan, secrets))
	var exit *ExitError
	if errors.As(err, &exit) {
		return operationError(err)
	}
	wait, cancel := context.WithTimeout(ctx, 240*time.Second)
	defer cancel()
	for {
		actual, readErr := c.ReadConfiguration(wait, config)
		if readErr == nil && SameConfiguration(actual, config) {
			if runtimeErr := c.verifyRuntimeFirewall(wait); runtimeErr != nil {
				return runtimeErr
			}
			_, confirmErr := c.runner.Run(wait, configurationConfirmScript(token, safetyConfig, secrets))
			if confirmErr == nil {
				return nil
			}
			if errors.As(confirmErr, &exit) && exit.Code == 77 {
				return operationError(confirmErr)
			}
		}
		select {
		case <-wait.Done():
			return fmt.Errorf("The operation outcome could not be confirmed. Recovery or confirmed cleanup may still be pending. Inspect router transaction results and connectivity before retrying: %w", wait.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

func sameSectionExceptManagedMarker(a, b Section) bool {
	left, right := a, b
	left.Options = copyOptions(a.Options)
	right.Options = copyOptions(b.Options)
	if left.Options != nil {
		delete(left.Options, ManagedOption)
	}
	if right.Options != nil {
		delete(right.Options, ManagedOption)
	}
	return reflect.DeepEqual(left, right)
}

func (c *Client) stableConfiguration(ctx context.Context, config Configuration, secrets ...map[string]string) (Configuration, bool, error) {
	output, err := c.runner.Run(ctx, configurationStableReadScript(config, secrets...))
	if err != nil {
		return nil, false, operationError(err)
	}
	var observed struct {
		Configuration Configuration `json:"configuration"`
		KeysMatch     bool          `json:"keys_match"`
	}
	if json.Unmarshal(output, &observed) != nil || len(observed.Configuration) != len(config) {
		return nil, false, errors.New("The router returned invalid configuration data.")
	}
	for i := range observed.Configuration {
		if len(observed.Configuration[i].Options) == 0 {
			observed.Configuration[i].Options = nil
		}
		if len(observed.Configuration[i].Lists) == 0 {
			observed.Configuration[i].Lists = nil
		}
	}
	return observed.Configuration, observed.KeysMatch, nil
}

var allowedOptions = func() map[string]map[string]bool {
	definitions := map[string]string{
		"network.device":            "name type bridge_empty ports stp igmp_snooping",
		"network.wireguard":         "public_key allowed_ips endpoint_host endpoint_port persistent_keepalive route_allowed_ips description",
		"network.interface":         "device proto disabled ipaddr netmask gateway dns peerdns defaultroute mtu addresses fwmark listen_port nohostroute vlan_id display_name home_iac_owner force_link delegate",
		"wireless.wifi-iface":       "disabled ssid encryption hidden isolate ifname device network mode macaddr wds ieee80211w wpa_group_rekey",
		"dhcp.dhcp":                 "interface start limit leasetime dhcpv4 dhcpv6 ra force ignore dynamicdhcp instance",
		"dhcp.host":                 "mac ip name dns leasetime duid",
		"dhcp.dnsmasq":              "noresolv server domain local rebind_domain cachesize",
		"firewall.zone":             "name network input output forward enabled masq mtu_fix log log_limit conntrack device subnet extra_src extra_dest",
		"firewall.forwarding":       "src dest enabled",
		"firewall.redirect":         "name src src_dport dest dest_ip dest_port proto target enabled reflection reflection_src helper",
		"firewall.rule":             "name src dest src_ip dest_ip proto src_port dest_port family target enabled src_mac icmp_type limit limit_burst weekdays start_date stop_date utc_time mark set_mark set_xmark helper log log_limit",
		"gl-dns-v2.dns":             "provider proto servers_list mode manual_enable manual_type manual_local_dns override_vpn force_dns",
		"eth_ports_config_map.port": "mode vlan_mode pvid port",
	}
	result := map[string]map[string]bool{}
	for scope, names := range definitions {
		result[scope] = map[string]bool{"home_iac_owner": true}
		for _, name := range strings.Fields(names) {
			result[scope][name] = true
		}
	}
	return result
}()
