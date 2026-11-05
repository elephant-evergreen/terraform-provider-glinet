package router

import (
	"encoding/json"
	"strings"
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

func configurationCleanup(token string) string {
	return "if [ -f " + quote(recoveryService) + " ]; then " + recoveryService + " disable; fi\nrm -f " + quote(recoveryService) + "\nrm -rf " + quote(transactionDir) + "\nrmdir " + quote(configurationLock+"-actor-"+token) + "\nrmdir " + quote(configurationLock) + "\n"
}

func configurationStableReadScript(config Configuration, keys map[string]string) string {
	data, _ := json.Marshal(keys)
	check := "local keys=json.decode(" + luaQuote(string(data)) + ")\nlocal equal=true\nif type(keys)=='table' then for name,value in pairs(keys) do if uci:get('wireless',name,'key')~=value then equal=false end end end\nio.write(json.encode({configuration=result,keys_match=equal}))"
	read := strings.Replace(configurationLua(config, false, nil), "io.write(json.encode(result))", check, 1)
	return preflight +
		"test ! -d " + quote(transactionDir) + " || exit 73\ntest ! -e " + quote(recoveryService) + " && test ! -L " + quote(recoveryService) + " || exit 73\n" +
		"test ! -d /tmp/home-network-iac.lock || exit 73\nmkdir " + quote(configurationLock) + " 2>/dev/null || exit 73\ntrap 'rmdir " + quote(configurationLock) + "' EXIT\npending=$(uci changes)\ntest -z \"$pending\" || exit 74\n" + read
}

func configurationApplyScript(config Configuration, keys map[string]string, token string, reboot bool) string {
	dir := transactionDir
	restore := "#!/bin/sh\nset -eu\nexec >> " + quote(dir+"/rollback.log") + " 2>&1\n"
	if reboot {
		restore += "if [ -f " + quote(dir+"/restored_boot") + " ] && [ \"$(cat /proc/sys/kernel/random/boot_id)\" != \"$(cat " + quote(dir+"/restored_boot") + ")\" ]; then\n" + configurationCleanup(token) + "exit 0\nfi\n"
	}
	restore += "for package in " + configurationPackages + "; do\ncp " + quote(dir) + "/\"$package\" /etc/config/\"$package\"\nuci -q revert \"$package\"\ndone\n" + configurationReload(reboot)
	if reboot {
		restore += "cat /proc/sys/kernel/random/boot_id > " + quote(dir+"/restored_boot") + "\nsync\nreboot\n"
	} else {
		restore += configurationCleanup(token)
	}
	watchdog := `#!/bin/sh
trap '' HUP
set -eu
umask 077
cd ` + quote(dir) + `
test "$(cat token)" = ` + quote(token) + ` || exit 75
mkdir ` + quote(configurationLock+"-actor-"+token) + ` 2>/dev/null || exit 73
exec >> rollback.log 2>&1
read current_boot < /proc/sys/kernel/random/boot_id
read initial_boot < boot
touch armed
if [ "$current_boot" = "$initial_boot" ]; then
 remaining=300
 while [ ! -f activate ] && [ "$remaining" -gt 0 ]; do sleep 1; remaining=$((remaining-1)); done
 if [ -f activate ]; then
  if ! /bin/sh request; then ln recover decision 2>/dev/null || true; fi
 else
  ln recover decision 2>/dev/null || true
 fi
else
 if [ -f awaiting_boot ]; then touch ready; else ln recover decision 2>/dev/null || true; fi
fi
remaining=300
while [ ! -f decision ] && [ "$remaining" -gt 0 ]; do sleep 1; remaining=$((remaining-1)); done
ln recover decision 2>/dev/null || true
case "$(cat decision)" in
 confirm)
 ` + configurationCleanup(token) + ` ;;
 rollback)
 while ! /bin/sh restore; do sleep 5; done
 ;;
 *) exit 75 ;;
esac
`

	service := "#!/bin/sh /etc/rc.common\nSTART=99\nstart() {\nmkdir -p " + quote(configurationLock) + "\nrm -f " + quote(dir+"/pid") + "\nstart-stop-daemon -S -b -m -p " + quote(dir+"/pid") + " -x /bin/sh -- " + quote(dir+"/rollback") + "\n}\n"
	request := "#!/bin/sh\ntrap '' HUP\nset -eu\ncd " + quote(dir) + "\nexec > apply.log 2>&1\n" + configurationLua(config, true, keys) + "fw3 -q print >/dev/null\nfor package in " + configurationPackages + "; do uci commit \"$package\"; done\n" + configurationReload(reboot)
	if reboot {
		request += "touch awaiting_boot\nsync\nreboot\n"
	} else {
		request += "touch ready\n"
	}

	script := preflight + `test ! -d /tmp/home-network-iac.lock || exit 73
pending=$(uci changes)
test -z "$pending" || exit 74
command -v lua >/dev/null
command -v start-stop-daemon >/dev/null
command -v fw3 >/dev/null
test "$(uci -q get adguardhome.config.enabled || true)$(uci -q get adguardhome.config.dns_enabled || true)" != '11' || exit 65
test ! -e ` + quote(recoveryService) + ` && test ! -L ` + quote(recoveryService) + ` || exit 73
umask 077
mkdir ` + quote(configurationLock) + ` 2>/dev/null || exit 73
transferred=0
prepared=0
trap 'if [ "$transferred" = 0 ]; then if [ "$prepared" = 1 ]; then ` + recoveryService + ` disable 2>/dev/null || true; rm -f ` + quote(recoveryService) + `; rm -rf ` + quote(dir) + `; fi; rmdir ` + quote(configurationLock) + `; fi' EXIT
mkdir ` + quote(dir) + ` 2>/dev/null || exit 73
prepared=1
cd ` + quote(dir) + `
printf '%s' ` + quote(token) + ` > ` + quote(dir+"/token") + `
printf confirm > ` + quote(dir+"/confirm") + `
printf rollback > ` + quote(dir+"/recover") + `
cat /proc/sys/kernel/random/boot_id > ` + quote(dir+"/boot") + `
for package in ` + configurationPackages + `; do cp /etc/config/"$package" ` + quote(dir) + `/"$package"; done
printf '%s' ` + quote(restore) + ` > ` + quote(dir+"/restore") + `
printf '%s' ` + quote(watchdog) + ` > ` + quote(dir+"/rollback") + `
cat > ` + quote(dir+"/request") + ` <<'GLINET_REQUEST'
` + request + `GLINET_REQUEST
printf '%s' ` + quote(service) + ` > ` + quote(recoveryService) + `
chmod 700 ` + quote(recoveryService) + `
` + recoveryService + ` enable
transferred=1
start-stop-daemon -S -b -m -p ` + quote(dir+"/pid") + ` -x /bin/sh -- ` + quote(dir+"/rollback") + `
for attempt in 1 2 3 4 5; do test ! -f armed || break; sleep 1; done
test -f armed || exit 76
touch activate
`

	return script
}

func configurationConfirmScript(token string, config Configuration, keys map[string]string) string {
	script := preflight + "cd " + quote(transactionDir) + "\n" + "test -d " + quote(configurationLock) + " || exit 75\ntest -f " + quote(transactionDir+"/ready") + " || exit 75\ntest \"$(cat " + quote(transactionDir+"/token") + ")\" = " + quote(token) + " || exit 75\nfw3 -q print >/dev/null\n"
	for _, s := range config {
		if s.Absent {
			continue
		}
		if s.Package == "network" && s.Type == "interface" && s.Options["disabled"] == "0" {
			script += "test \"$(ubus call " + quote("network.interface."+s.Name) + " status | jsonfilter -e '@.up')\" = 'true' || exit 75\n"
		}
		if s.Package == "wireless" && s.Type == "wifi-iface" && s.Options["disabled"] == "0" && s.Options["ifname"] != "" && s.Options["network"] != "" {
			script += "test \"$(basename \"$(readlink " + quote("/sys/class/net/"+s.Options["ifname"]+"/master") + ")\")\" = " + quote("br-"+s.Options["network"]) + " || exit 75\n"
		}
		if s.Package == "network" && s.Type == "device" && s.Options["name"] != "" {
			for _, port := range s.Lists["ports"] {
				script += "test \"$(basename \"$(readlink " + quote("/sys/class/net/"+port+"/master") + ")\")\" = " + quote(s.Options["name"]) + " || exit 75\n"
			}
		}
		if s.Package == "gl-dns-v2" && s.Options["mode"] == "secure" && s.Options["provider"] != "" && len(s.Lists["servers_list"]) > 0 {
			data, _ := json.Marshal(s)
			script += "pidof dnsproxy >/dev/null\nlua - <<'GLINET_DNS_LUA'\nlocal json=require('cjson')\nlocal s=json.decode(" + luaQuote(string(data)) + ")\n" + dnsLiveCheck + "GLINET_DNS_LUA\n"
		}
	}
	for name, value := range keys {
		script += "test \"$(uci -q get " + quote("wireless."+name+".key") + ")\" = " + quote(value) + " || exit 75\n"
	}
	return script + "ln confirm decision 2>/dev/null || test \"$(cat decision)\" = confirm || exit 75\nsync\nfor attempt in 1 2 3 4 5 6 7 8 9 10; do\n[ -f " + quote(transactionDir+"/token") + " ] || exit 0\n[ \"$(cat " + quote(transactionDir+"/token") + ")\" = " + quote(token) + " ] || exit 0\nsleep 1\ndone\nexit 75\n"
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
