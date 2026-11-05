package router

import (
	"context"
	"crypto/rand"
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
	Package string              `json:"package"`
	Name    string              `json:"name"`
	Type    string              `json:"type"`
	Match   map[string]string   `json:"match,omitempty"`
	Options map[string]string   `json:"options,omitempty"`
	Lists   map[string][]string `json:"lists,omitempty"`
	Absent  bool                `json:"absent,omitempty"`
}

type Configuration []Section

var packages = map[string]bool{"network": true, "wireless": true, "dhcp": true, "firewall": true, "gl-dns-v2": true, "eth_ports_config_map": true}
var selector = regexp.MustCompile(`^@([A-Za-z0-9_-]+)\[[0-9]+\]$`)
var optionName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func DecodeConfiguration(value string) (Configuration, error) {
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
	for index, s := range config {
		anonymous := selector.FindStringSubmatch(s.Name)
		if !packages[s.Package] || (!ValidSection(s.Name) && anonymous == nil) || !sectionName.MatchString(s.Type) && !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(s.Type) {
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
		if anonymous != nil && (anonymous[1] != s.Type || s.Absent) {
			return nil, errors.New("Anonymous selectors must match the section type and cannot be deleted.")
		}
		if s.Absent && (len(s.Options)+len(s.Lists)+len(s.Match) > 0) {
			return nil, errors.New("Absent sections cannot include options, lists or matches.")
		}
		for k, v := range s.Options {
			if !validOption(k, v) {
				return nil, errors.New("Invalid or secret UCI option. Use wireless_keys_wo for Wi-Fi keys.")
			}
			if _, ok := s.Lists[k]; ok {
				return nil, errors.New("An option cannot be both a scalar and a list.")
			}
		}
		for k, v := range s.Match {
			if !validOption(k, v) {
				return nil, errors.New("Invalid UCI identity match.")
			}
		}
		for k, vs := range s.Lists {
			if len(vs) == 0 {
				return nil, errors.New("UCI lists must contain at least one item.")
			}
			for _, v := range vs {
				if !validOption(k, v) {
					return nil, errors.New("Invalid or secret UCI list option.")
				}
			}
		}
	}
	return config, nil
}

func validOption(key, value string) bool {
	lower := strings.ToLower(key)
	return optionName.MatchString(key) && lower != "key" && !strings.Contains(lower, "password") && !strings.Contains(lower, "secret") && !strings.Contains(lower, "token") && !strings.Contains(lower, "private_key") && len(value) <= 1024 && !strings.ContainsFunc(value, unicode.IsControl)
}

func luaQuote(value string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\r", "\\r").Replace(value) + "'"
}

func configurationLua(config Configuration, write bool, keys map[string]string) string {
	data, _ := json.Marshal(config)
	source := "local json=require('cjson')\nlocal uci=require('uci').cursor()\nlocal spec=json.decode(" + luaQuote(string(data)) + ")\n" + `local result={}
local rule_order={}
local rule_slots={}
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
  if #matches~=1 then os.exit(65) end
  name=matches[1]
 end
 local kind=uci:get(s.package,name)
 if kind and kind~=s.type then os.exit(65) end
 for k,want in pairs(s.match or {}) do
  if not kind or uci:get(s.package,name,k)~=want then os.exit(65) end
 end
 local actual={package=s.package,name=s.name,type=s.type,match=s.match,absent=s.absent,options={},lists={}}
`
	if write {
		source += ` if s.absent then
  if kind then assert(uci:delete(s.package,name)) end
 else
  if not kind then assert(uci:set(s.package,name,s.type)) end
  for k,v in pairs(s.options or {}) do assert(uci:set(s.package,name,k,v)) end
  for k,v in pairs(s.lists or {}) do assert(uci:set(s.package,name,k,v)) end
 end
`
	} else {
		source += ` if s.absent then
  actual.absent=not kind
 else
  if not kind then actual.type='' end
  for k,v in pairs(s.options or {}) do
   local value=uci:get(s.package,name,k)
   if type(value)~='string' then value='' end
   actual.options[k]=value
  end
  for k,v in pairs(s.lists or {}) do
   local value=uci:get(s.package,name,k)
   if type(value)~='table' then value=json.null end
   actual.lists[k]=value
  end
 end
 result[i]=actual
 if s.package=='firewall' and s.type=='rule' and not s.absent then
  rule_order[#rule_order+1]={value=actual,index=rule_indices[name] or 2147483647}
  rule_slots[#rule_slots+1]=i
 end
`
	}
	source += "end\n"
	if write {
		for name, value := range keys {
			source += "assert(uci:get('wireless'," + luaQuote(name) + ")=='wifi-iface')\nassert(uci:set('wireless'," + luaQuote(name) + ",'key'," + luaQuote(value) + "))\n"
		}
		source += "local count=0\nuci:foreach('firewall',nil,function() count=count+1 end)\nfor _,s in ipairs(spec) do if s.package=='firewall' and s.type=='rule' and not s.absent then assert(uci:reorder(s.package,s.name,count-1)) end end\n"
		source += "for _,p in ipairs({'network','wireless','dhcp','firewall','gl-dns-v2','eth_ports_config_map'}) do assert(uci:save(p)) end\n"
	} else {
		source += "table.sort(rule_order,function(a,b) return a.index<b.index end)\nfor i,slot in ipairs(rule_slots) do result[slot]=rule_order[i].value end\nio.write(json.encode(result))\n"
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
		if !found || len(key) < 8 || len(key) > 63 || strings.ContainsFunc(key, func(r rune) bool { return r < 32 || r > 126 }) {
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
	actual, err := c.ReadConfiguration(ctx, config)
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		check := preflight
		for name, value := range keys {
			check += "test \"$(uci -q get " + quote("wireless."+name+".key") + " || true)\" = " + quote(value) + " || exit 77\n"
		}
		_, keyErr := c.runner.Run(ctx, check)
		if keyErr == nil {
			keys = nil
		} else {
			var status *ExitError
			if !errors.As(keyErr, &status) || status.Code != 77 {
				return operationError(keyErr)
			}
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
	reboot := false
	for i, s := range config {
		if !reflect.DeepEqual(s, actual[i]) && (s.Package == "network" || s.Package == "wireless") {
			reboot = true
		}
	}
	if len(keys) > 0 {
		reboot = true
	}
	_, err = c.runner.Run(ctx, configurationApplyScript(config, keys, token, reboot))
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
		}
		select {
		case <-wait.Done():
			return fmt.Errorf("Configuration was not confirmed. Router-side recovery restores the previous configuration after 300 seconds, including after reboot. Inspect connectivity before retrying: %w", wait.Err())
		case <-time.After(2 * time.Second):
		}
	}
}
