package router

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func DecodeConfiguration(value string) (Configuration, error) {
	return decodeConfiguration(value, false)
}
func DecodeObservedConfiguration(value string) (Configuration, error) {
	return decodeConfiguration(value, true)
}
func decodeConfiguration(value string, observed bool) (Configuration, error) {
	if len(value) > 262144 {
		return nil, errors.New("Configuration exceeds 256 KiB.")
	}
	var config Configuration
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, errors.New("configuration must be a JSON array of UCI section objects.")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("configuration must contain exactly one JSON value.")
	}
	if len(config) == 0 || len(config) > 256 {
		return nil, errors.New("Configure between 1 and 256 UCI sections.")
	}
	seen := map[string]bool{}
	positions := map[int]bool{}
	rules, positioned := 0, 0
	for index, s := range config {
		if s.Missing && !observed {
			return nil, errors.New("missing is reserved for observed drift.")
		}
		anonymous := selector.FindStringSubmatch(s.Name)
		if allowedOptions[s.Package+"."+s.Type] == nil || (!ValidSection(s.Name) && anonymous == nil) {
			return nil, errors.New("Unsupported UCI package, section name or section type.")
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
			return nil, errors.New("Each UCI section may appear only once.")
		}
		seen[key] = true
		if anonymous == nil && len(s.Match) > 0 {
			return nil, errors.New("Identity matches are supported only for anonymous sections.")
		}
		if anonymous != nil && (anonymous[1] != s.Type || s.Absent) {
			return nil, errors.New("Anonymous selectors must match the section type and cannot be deleted.")
		}
		if s.Position != nil {
			if s.Package != "firewall" || s.Type != "rule" || (!observed && (*s.Position < 0 || *s.Position > 4095)) {
				return nil, errors.New("position must be a firewall rule index between 0 and 4095.")
			}
			if !observed && positions[*s.Position] {
				return nil, errors.New("Firewall rule positions must be unique.")
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
			return nil, errors.New("Absent sections cannot include options, lists or matches.")
		}
		for k, v := range s.Options {
			if !observed && s.Package == "wireless" && ((k == "ssid" && !ValidSSID(v)) || ((k == "disabled" || k == "hidden" || k == "isolate") && v != "0" && v != "1")) {
				return nil, errors.New("Wi-Fi SSIDs must contain 1–32 UTF-8 bytes and flags must be 0 or 1.")
			}
			if !allowedOptions[s.Package+"."+s.Type][k] || (!observed && !validOption(s.Package, s.Type, k, v)) {
				return nil, errors.New("Invalid or secret UCI option. Use wireless_keys_wo for Wi-Fi keys.")
			}
			if _, ok := s.Lists[k]; ok {
				return nil, errors.New("An option cannot be both a scalar and a list.")
			}
		}
		for k, v := range s.Match {
			if value, ok := s.Options[k]; !observed && ok && value != v {
				return nil, errors.New("Managed identity options must equal their match values.")
			}
			if _, ok := s.Lists[k]; ok {
				return nil, errors.New("Identity match options cannot be lists.")
			}
			if !validOption(s.Package, s.Type, k, v) {
				return nil, errors.New("Invalid UCI identity match.")
			}
		}
		for k, vs := range s.Lists {
			if s.Package == "wireless" && !observed {
				return nil, errors.New("Wi-Fi attributes must use scalar options, not lists.")
			}
			if !allowedOptions[s.Package+"."+s.Type][k] {
				return nil, errors.New("Unsupported UCI list option.")
			}
			if len(vs) == 0 && !observed {
				return nil, errors.New("UCI lists must contain at least one item.")
			}
			for _, v := range vs {
				if !allowedOptions[s.Package+"."+s.Type][k] || (!observed && !validOption(s.Package, s.Type, k, v)) {
					return nil, errors.New("Invalid or secret UCI list option.")
				}
			}
		}
	}
	if positioned > 0 && positioned != rules {
		return nil, errors.New("Set positions for all managed firewall rules or leave all positions unspecified.")
	}
	return config, nil
}

func validOption(pkg, kind, key, value string) bool {
	return allowedOptions[pkg+"."+kind][key] && len(value) <= 1024 && !strings.ContainsFunc(value, unicode.IsControl)
}

func luaQuote(value string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\r", "\\r").Replace(value) + "'"
}

func configurationLua(config Configuration, write bool, keys map[string]string) string {
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
	keyNames := map[string]bool{}
	for name := range keys {
		keyNames[name] = true
	}
	available, _ := json.Marshal(keyNames)
	source := "local json=require('cjson')\nlocal uci=require('uci').cursor()\nlocal writing=" + fmt.Sprint(write) + "\nlocal spec=json.decode(" + luaQuote(string(data)) + ")\nlocal key_names=json.decode(" + luaQuote(string(available)) + ")\n" + `local result={}
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
 if writing and s.package=='wireless' and not s.absent and not kind and not key_names[s.name] then os.exit(65) end
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
  for k,v in pairs(s.options or {}) do assert(uci:set(s.package,name,k,v)) end
  for k,v in pairs(s.lists or {}) do assert(uci:set(s.package,name,k,v)) end
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
   if type(value)~='table' then value=json.null end
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
		for name, value := range keys {
			source += "assert(uci:get('wireless'," + luaQuote(name) + ")=='wifi-iface')\nassert(uci:set('wireless'," + luaQuote(name) + ",'key'," + luaQuote(value) + "))\n"
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

func ValidWirelessKeys(config Configuration, keys map[string]string) bool {
	for name, key := range keys {
		found := false
		for _, s := range config {
			if s.Package == "wireless" && s.Name == name && s.Type == "wifi-iface" && !s.Absent {
				found = true
			}
		}
		if !ValidSection(name) || !found || len(key) < 8 || len(key) > 63 || strings.ContainsFunc(key, func(r rune) bool { return r < 32 || r > 126 }) {
			return false
		}
	}
	return true
}

func (c *Client) WriteConfiguration(ctx context.Context, config Configuration, keys map[string]string) error {
	if !ValidWirelessKeys(config, keys) {
		return errors.New("Wi-Fi keys must reference managed named wifi-iface sections and contain 8–63 printable ASCII characters.")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	actual, keysMatch, err := c.stableConfiguration(ctx, config, keys)
	if err != nil {
		return err
	}
	if keysMatch {
		keys = nil
	}
	for i, s := range config {
		if s.Package == "wireless" && !s.Absent && actual[i].Missing && keys[s.Name] == "" {
			return errors.New("A missing Wi-Fi section requires its write-only key and a new key generation before it can be restored.")
		}
		if s.Package == "firewall" && s.Type == "rule" && s.Position == nil && s.Name != actual[i].Name {
			return errors.New("Changing managed firewall rule order requires explicit positions for all managed rules.")
		}
	}
	if SameConfiguration(actual, config) && len(keys) == 0 {
		return nil
	}
	tokenBytes := make([]byte, 16)
	if _, err = rand.Read(tokenBytes); err != nil {
		return errors.New("Cannot initialize a configuration transaction.")
	}
	token := hex.EncodeToString(tokenBytes)
	plan := reloadPlan{}
	for i, s := range config {
		if reflect.DeepEqual(s, actual[i]) {
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
	if len(keys) > 0 {
		plan.Wireless = true
	}
	_, err = c.runner.Run(ctx, configurationApplyScript(config, keys, token, plan))
	var exit *ExitError
	if errors.As(err, &exit) {
		return operationError(err)
	}
	wait, cancel := context.WithTimeout(ctx, 240*time.Second)
	defer cancel()
	for {
		actual, readErr := c.ReadConfiguration(wait, config)
		if readErr == nil && SameConfiguration(actual, config) {
			_, confirmErr := c.runner.Run(wait, configurationConfirmScript(token, config, keys))
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

func (c *Client) stableConfiguration(ctx context.Context, config Configuration, keys map[string]string) (Configuration, bool, error) {
	output, err := c.runner.Run(ctx, configurationStableReadScript(config, keys))
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
		"network.device":            "name type bridge_empty ports",
		"network.interface":         "device proto disabled ipaddr netmask vlan_id display_name home_iac_owner force_link delegate",
		"wireless.wifi-iface":       "disabled ssid encryption hidden isolate ifname device network mode",
		"dhcp.dhcp":                 "interface start limit leasetime dhcpv4 dhcpv6 ra force",
		"dhcp.host":                 "mac ip name",
		"dhcp.dnsmasq":              "noresolv server",
		"firewall.zone":             "name network input output forward",
		"firewall.forwarding":       "src dest enabled",
		"firewall.rule":             "name src dest src_ip dest_ip proto dest_port family target enabled",
		"gl-dns-v2.dns":             "provider proto servers_list mode manual_enable manual_type manual_local_dns override_vpn force_dns",
		"eth_ports_config_map.port": "mode vlan_mode pvid port",
	}
	result := map[string]map[string]bool{}
	for scope, names := range definitions {
		result[scope] = map[string]bool{}
		for _, name := range strings.Fields(names) {
			result[scope][name] = true
		}
	}
	return result
}()

func (c *Client) StableConfiguration(ctx context.Context, config Configuration) (Configuration, error) {
	actual, _, err := c.stableConfiguration(ctx, config, nil)
	return actual, err
}
