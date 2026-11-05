package router

import (
	"encoding/json"
)

const transactionDir = "/root/.terraform-provider-glinet-configuration"
const configurationLock = "/tmp/terraform-provider-glinet-wifi"
const recoveryService = "/etc/init.d/terraform-glinet-recovery"
const configurationPackages = "network wireless dhcp firewall gl-dns-v2 eth_ports_config_map"

const dnsRefresh = `dns_config=$(ubus call gl-session call '{"module":"dns","func":"get_config"}' | jsonfilter -e '@.result')
test -n "$dns_config"
dns_result=$(ubus call gl-session call "{\"module\":\"dns\",\"func\":\"set_config\",\"params\":$dns_config}")
printf '%s' "$dns_result" | lua -e 'local v=require("cjson").decode(io.read("*a")); if type(v)~="table" or v.error or v.result==nil then os.exit(1) end'
`

func configurationReload(reboot bool) string {
	script := ""
	if reboot {
		script += "/etc/init.d/network reload\n/etc/init.d/gl-vlan-port-map sync\n"
	}
	return script + dnsRefresh + "/etc/init.d/dnsmasq restart\n/etc/init.d/firewall restart\n"
}

func configurationApplyScript(config Configuration, keys map[string]string, token string, reboot bool) string {
	dir := transactionDir
	rollback := `#!/bin/sh
set -u
umask 077
exec > ` + quote(dir+"/rollback.log") + ` 2>&1
sleep 300 &
child=$!
trap 'kill "$child" 2>/dev/null || true; exit 0' TERM INT
read current_boot < /proc/sys/kernel/random/boot_id
read initial_boot < ` + quote(dir+"/boot") + `
if [ "$current_boot" != "$initial_boot" ]; then touch ` + quote(dir+"/ready") + `; fi
touch ` + quote(dir+"/armed") + `
wait "$child" || exit 0
if mkdir ` + quote(dir+"/decision") + ` 2>/dev/null; then
 for package in ` + configurationPackages + `; do
  cp ` + quote(dir) + `/"$package" /etc/config/"$package" || exit 1
  uci -q revert "$package" || true
 done
 ` + recoveryService + ` disable || true
 rm -f ` + quote(recoveryService) + `
 touch ` + quote(dir+"/rolled_back") + `
 ` + configurationReload(reboot) + `
 rm -rf ` + quote(configurationLock) + `
`
	if reboot {
		rollback += "rm -rf " + quote(dir) + "\nsync\nreboot\n"
	} else {
		rollback += "rm -rf " + quote(dir) + "\n"
	}
	rollback += "fi\n"
	service := "#!/bin/sh /etc/rc.common\nSTART=99\nstart() {\nmkdir -p " + quote(configurationLock) + "\nrm -f " + quote(dir+"/pid") + "\nstart-stop-daemon -S -b -m -p " + quote(dir+"/pid") + " -x /bin/sh -- " + quote(dir+"/rollback") + "\n}\n"
	reload := "#!/bin/sh\nset -eu\nexec > " + quote(dir+"/reload.log") + " 2>&1\nsleep 2\n" + configurationReload(reboot)
	if reboot {
		reload += "sync\nreboot\n"
	} else {
		reload += "touch " + quote(dir+"/ready") + "\n"
	}
	script := preflight + `test ! -d /tmp/terraform-provider-glinet-wifi || exit 73
test ! -d /tmp/home-network-iac.lock || exit 73
pending=$(uci changes)
test -z "$pending" || exit 74
command -v lua >/dev/null
command -v start-stop-daemon >/dev/null
command -v fw3 >/dev/null
test "$(uci -q get adguardhome.config.enabled || true)$(uci -q get adguardhome.config.dns_enabled || true)" != '11' || exit 65
test ! -e ` + quote(recoveryService) + ` && test ! -L ` + quote(recoveryService) + ` || exit 73
umask 077
mkdir ` + quote(configurationLock) + ` 2>/dev/null || exit 73
armed=0
prepared=0
trap 'if [ "$armed" = 0 ]; then if [ "$prepared" = 1 ]; then ` + recoveryService + ` disable 2>/dev/null || true; rm -f ` + quote(recoveryService) + `; rm -rf ` + quote(dir) + `; fi; rmdir ` + quote(configurationLock) + `; fi' EXIT
mkdir ` + quote(dir) + ` 2>/dev/null || exit 73
prepared=1
test ! -e ` + quote(recoveryService) + ` && test ! -L ` + quote(recoveryService) + ` || exit 73
printf '%s' ` + quote(token) + ` > ` + quote(dir+"/token") + `
cat /proc/sys/kernel/random/boot_id > ` + quote(dir+"/boot") + `
for package in ` + configurationPackages + `; do cp /etc/config/"$package" ` + quote(dir) + `/"$package"; done
printf '%s' ` + quote(rollback) + ` > ` + quote(dir+"/rollback") + `
printf '%s' ` + quote(reload) + ` > ` + quote(dir+"/reload") + `
printf '%s' ` + quote(service) + ` > ` + quote(recoveryService) + `
chmod 700 ` + quote(recoveryService) + `
` + recoveryService + ` enable
start-stop-daemon -S -b -m -p ` + quote(dir+"/pid") + ` -x /bin/sh -- ` + quote(dir+"/rollback") + `
for attempt in 1 2 3 4 5; do test ! -f ` + quote(dir+"/armed") + ` || break; sleep 1; done
test -f ` + quote(dir+"/armed") + ` || exit 76
armed=1
` + configurationLua(config, true, keys) + `fw3 -q print >/dev/null
for package in ` + configurationPackages + `; do uci commit "$package"; done
start-stop-daemon -S -b -m -p ` + quote(dir+"/reload.pid") + ` -x /bin/sh -- ` + quote(dir+"/reload") + "\n"
	return script
}

func configurationConfirmScript(token string, config Configuration, keys map[string]string) string {
	script := preflight + "test -d " + quote(configurationLock) + " || exit 75\ntest -f " + quote(transactionDir+"/ready") + " || exit 75\ntest \"$(cat " + quote(transactionDir+"/token") + ")\" = " + quote(token) + " || exit 75\nfw3 -q print >/dev/null\n"
	for _, s := range config {
		if s.Absent {
			continue
		}
		if s.Package == "network" && s.Type == "interface" && s.Options["disabled"] == "0" {
			script += "test \"$(ubus call " + quote("network.interface."+s.Name) + " status | jsonfilter -e '@.up')\" = 'true' || exit 75\n"
		}
		if s.Package == "wireless" && s.Type == "wifi-iface" && s.Options["disabled"] == "0" && s.Options["ifname"] != "" && s.Options["network"] != "" {
			script += "test \"$(readlink " + quote("/sys/class/net/"+s.Options["ifname"]+"/master") + ")\" = " + quote("../../devices/virtual/net/br-"+s.Options["network"]) + " || test \"$(basename \"$(readlink " + quote("/sys/class/net/"+s.Options["ifname"]+"/master") + ")\")\" = " + quote("br-"+s.Options["network"]) + " || exit 75\n"
		}
	}
	for name, value := range keys {
		script += "test \"$(uci -q get " + quote("wireless."+name+".key") + ")\" = " + quote(value) + " || exit 75\n"
	}
	for _, s := range config {
		if s.Package == "gl-dns-v2" && s.Options["mode"] == "secure" && s.Options["provider"] != "" && len(s.Lists["servers_list"]) > 0 {
			data, _ := json.Marshal(s)
			script += "pidof dnsproxy >/dev/null\nlua - <<'GLINET_DNS_LUA'\nlocal json=require('cjson')\nlocal s=json.decode(" + luaQuote(string(data)) + ")\n" + dnsLiveCheck + "GLINET_DNS_LUA\n"
		}
	}
	return script + "mkdir " + quote(transactionDir+"/decision") + " 2>/dev/null || exit 75\nstart-stop-daemon -K -p " + quote(transactionDir+"/pid") + " || true\n" + recoveryService + " disable || true\nrm -f " + quote(recoveryService) + "\nrm -rf " + quote(transactionDir) + " " + quote(configurationLock) + "\n"
}

func configurationJSON(config Configuration) string {
	data, _ := json.Marshal(config)
	return string(data)
}

const dnsLiveCheck = `local provider=io.open('/etc/dnsproxy/provider/'..s.options.provider)
if not provider then os.exit(75) end
local metadata=json.decode(provider:read('*a')); provider:close()
local expected={}
for _,name in ipairs(s.lists.servers_list) do
 local endpoint=nil
 for _,entry in ipairs(metadata.server_list or {}) do if entry.name==name then endpoint=entry['url_'..s.options.proto] end end
 if not endpoint then os.exit(75) end
 expected[endpoint]=true
end
local file=io.open('/etc/dnsproxy/dnsproxy.yaml')
if not file then os.exit(75) end
local active=false
local observed={}
for line in file:lines() do
 if line=='upstream:' then active=true
 elseif active then
  if line:match('^%S') then break end
  local value=line:match('^%s*%-%s*(.+)%s*$')
  if value then local ok,endpoint=pcall(json.decode,value); if not ok or not expected[endpoint] then os.exit(75) end; observed[endpoint]=true end
 end
end
file:close()
for endpoint in pairs(expected) do if not observed[endpoint] then os.exit(75) end end
`
