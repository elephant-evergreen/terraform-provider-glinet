package router

import (
	"encoding/json"
	"strings"
)

const transactionDir = "/root/.terraform-provider-glinet-configuration"
const resultDir = "/root/.terraform-provider-glinet-results"
const configurationLock = "/tmp/terraform-provider-glinet-wifi"
const recoveryService = "/etc/init.d/terraform-glinet-recovery"
const configurationPackages = "network wireless dhcp firewall gl-dns-v2 eth_ports_config_map system dropbear upnpd sqm adguardhome tailscale zerotier tor openvpn wireguard qos minidlna uhttpd kmwan route_policy gl_ddns samba4 gl-mesh repeater gl_timer gl-black_white_list wan-access parental_control_v2 wireguard_server gl_s2s"

const dnsRefresh = `dns_config=$(ubus call gl-session call '{"module":"dns","func":"get_config"}' | jsonfilter -e '@.result')
test -n "$dns_config"
dns_result=$(ubus call gl-session call "{\"module\":\"dns\",\"func\":\"set_config\",\"params\":$dns_config}")
printf '%s' "$dns_result" | lua -e 'local v=require("cjson").decode(io.read("*a")); if type(v)~="table" or v.error or v.result==nil then os.exit(1) end'
`

type reloadPlan struct {
	Reboot          bool
	Wireless        bool
	DNS             bool
	DHCP            bool
	Firewall        bool
	Cloud           bool
	CloudEnabled    bool
	ServiceSnapshot bool
}

func configurationReload(plan reloadPlan) string {
	script := ""
	if plan.Reboot {
		script += "/etc/init.d/network reload\n/etc/init.d/gl-vlan-port-map sync\n"
	} else if plan.Wireless {
		script += "wifi reload\n"
	}
	if plan.DNS {
		script += dnsRefresh
	}
	if plan.Reboot || plan.DNS || plan.DHCP {
		script += "/etc/init.d/dnsmasq restart\n"
	}
	if plan.Reboot || plan.Firewall {
		script += "/etc/init.d/firewall restart\n"
	}
	return script
}

func cloudSetConfigScript(enabled bool) string {
	value := "false"
	if enabled {
		value = "true"
	}
	return "ubus call gl-session call " + quote(`{"module":"cloud","func":"set_config","params":{"cloud_enable":`+value+`}}`) + "\n"
}

func cloudVerifyScript(enabled bool) string {
	value := "false"
	if enabled {
		value = "true"
	}
	script := "test \"$(ubus call gl-session call " + quote(`{"module":"cloud","func":"get_config"}`) + " | jsonfilter -e '@.result.cloud_enable')\" = '" + value + "'\n"
	if !enabled {
		for _, name := range []string{"gl-cloud", "rtty"} {
			script += "for _ in 1 2 3 4 5 6 7 8 9 10; do\n if ! pidof " + name + " >/dev/null 2>&1; then break; fi\n sleep 1\ndone\nif pidof " + name + " >/dev/null 2>&1; then exit 75; fi\n"
		}
	}
	return script
}

func cloudSnapshotScript() string {
	return `printf 'gl_cloud_running=%s\n' "$(if pidof gl-cloud >/dev/null 2>&1; then echo 1; else echo 0; fi)"
printf 'gl_cloud_registered=%s\n' "$(if ubus call service list '{"name":"gl-cloud"}' 2>/dev/null | grep -q '"gl-cloud"'; then echo 1; else echo 0; fi)"
printf 'rtty_running=%s\n' "$(if pidof rtty >/dev/null 2>&1; then echo 1; else echo 0; fi)"
printf 'rtty_registered=%s\n' "$(if ubus call service list '{"name":"rtty"}' 2>/dev/null | grep -q '"rtty"'; then echo 1; else echo 0; fi)"
printf 'mptun_running=%s\n' "$(if pidof mptun >/dev/null 2>&1; then echo 1; else echo 0; fi)"
printf 'mptun_registered=%s\n' "$(if ubus call service list '{"name":"mptun"}' 2>/dev/null | grep -q '"mptun"'; then echo 1; else echo 0; fi)"
printf 'mpifd_running=%s\n' "$(if pidof mpifd >/dev/null 2>&1; then echo 1; else echo 0; fi)"
printf 'mpifd_registered=%s\n' "$(if ubus call service list '{"name":"mpifd"}' 2>/dev/null | grep -q '"mpifd"'; then echo 1; else echo 0; fi)"
`
}

func cloudRestoreScript() string {
	return `restore_service() {
 name=$1
 running=$2
 registered=$3
 init=$4
 if [ "$running" = 1 ]; then
  "$init" start >/dev/null 2>&1 || restore_failed=1
  for _ in 1 2 3; do
   if pidof "$name" >/dev/null 2>&1; then break; fi
   sleep 1
  done
  if ! pidof "$name" >/dev/null 2>&1; then restore_failed=1; fi
 elif [ "$registered" = 1 ]; then
  ubus call service add "{\"name\":\"$name\",\"instances\":{}}" >/dev/null 2>&1 || restore_failed=1
  if pidof "$name" >/dev/null 2>&1; then restore_failed=1; fi
 else
  "$init" stop >/dev/null 2>&1 || true
  for _ in 1 2 3; do
   if ! pidof "$name" >/dev/null 2>&1; then break; fi
   sleep 1
  done
  if pidof "$name" >/dev/null 2>&1; then restore_failed=1; fi
  ubus call service delete "{\"name\":\"$name\"}" >/dev/null 2>&1 || true
 fi
 if [ "$registered" = 1 ]; then
  ubus call service list "{\"name\":\"$name\"}" 2>/dev/null | grep -q "\"$name\"" || restore_failed=1
 else
  if ubus call service list "{\"name\":\"$name\"}" 2>/dev/null | grep -q "\"$name\""; then restore_failed=1; fi
 fi
}
restore_failed=0
. ./service-before
restore_service gl-cloud "$gl_cloud_running" "$gl_cloud_registered" /etc/init.d/gl-cloud
restore_service rtty "$rtty_running" "$rtty_registered" /etc/init.d/rtty
restore_service mptun "$mptun_running" "$mptun_registered" /etc/init.d/mptun
restore_service mpifd "$mpifd_running" "$mpifd_registered" /etc/init.d/mpifd
printf '%s' "$restore_failed" > service-restore
if [ "$restore_failed" != 0 ]; then
 exit 1
fi
`
}

func cloudCommitPackages(plan reloadPlan) string {
	if plan.ServiceSnapshot {
		return " gl-cloud"
	}
	return ""
}

func cloudRequest(plan reloadPlan) string {
	if plan.Cloud {
		return cloudSetConfigScript(plan.CloudEnabled) + cloudVerifyScript(plan.CloudEnabled)
	}
	return ""
}

func configurationOutcome(token, outcome string) string {
	result := resultDir + "/" + token
	if outcome == "" {
		return "test -f " + quote(result) + " || exit 75\n"
	}
	return "mkdir -p " + quote(resultDir) + "\nif [ ! -f " + quote(result) + " ]; then printf '%s' " + quote(outcome) + " > " + quote(result+".tmp") + "; mv " + quote(result+".tmp") + " " + quote(result) + "; sync; fi\ntest \"$(cat " + quote(result) + ")\" = " + quote(outcome) + " || exit 75\n"
}

func configurationCleanup(token, outcome string) string {
	retired := transactionDir + "-retired-" + token
	return "if [ -e " + quote(configurationLock+"/token") + " ]; then test \"$(cat " + quote(configurationLock+"/token") + ")\" = " + quote(token) + " || exit 75; fi\n" + configurationOutcome(token, outcome) + "if [ -d " + quote(transactionDir) + " ]; then test \"$(cat " + quote(transactionDir+"/token") + ")\" = " + quote(token) + " || exit 75; mv " + quote(transactionDir) + " " + quote(retired) + "; fi\nsync\nrm -rf " + quote(retired) + "\nsync\nrm -f " + quote(recoveryService) + "\nsync\nrm -f /etc/rc.d/S??terraform-glinet-recovery /etc/rc.d/K??terraform-glinet-recovery\nrm -f " + quote(configurationLock+"/token") + "\nrmdir " + quote(configurationLock) + " 2>/dev/null || true\nrmdir " + quote(configurationLock+"-actor-"+token) + " 2>/dev/null || true\n"
}

func configurationStableReadScript(config Configuration, secrets ...map[string]string) string {
	secretData := "{}"
	if len(secrets) > 0 {
		encoded, _ := json.Marshal(secrets[0])
		secretData = string(encoded)
	}
	check := "local secrets=json.decode(" + luaQuote(secretData) + ")\nlocal equal=true\nif type(secrets)=='table' then for key,value in pairs(secrets) do local p,s,o=key:match('^([^.]+)%.([^.]+)%.(.+)$'); if not p or uci:get(p,s,o)~=value then equal=false end end end\nio.write(json.encode({configuration=result,keys_match=equal}))"
	read := strings.Replace(configurationLua(config, false), "io.write(json.encode(result))", check, 1)
	return preflight +
		"test ! -d " + quote(transactionDir) + " || exit 73\ntest ! -e " + quote(recoveryService) + " && test ! -L " + quote(recoveryService) + " || exit 73\n" +
		configurationDNSOwnership(config) + "mkdir " + quote(configurationLock) + " 2>/dev/null || exit 73\ntrap 'rmdir " + quote(configurationLock) + "' EXIT\npending=$(uci changes)\ntest -z \"$pending\" || exit 74\n" + read
}

func configurationApplyScript(config Configuration, token string, plan reloadPlan, secrets ...map[string]string) string {
	dir := transactionDir
	serviceSnapshot := ""
	servicePackages := ""
	if plan.Cloud {
		serviceSnapshot = "{ " + cloudSnapshotScript() + "} > service-before\n"
	}
	if plan.ServiceSnapshot {
		servicePackages = "cp /etc/config/gl-cloud " + quote(dir) + "/gl-cloud\nprintf '%s\\n' gl-cloud > " + quote(dir) + "/packages.extra\n"
		if plan.Cloud {
			servicePackages += "if [ -f /etc/config/mptun ] && [ \"$(uci -q get mptun.global.enable)\" = 1 ]; then cp /etc/config/mptun " + quote(dir) + "/mptun\nprintf '%s\\n' mptun >> " + quote(dir) + "/packages.extra\nfi\n"
		}
	}
	restore := "#!/bin/sh\nset -eu\nexec >> " + quote(dir+"/rollback.log") + " 2>&1\n" + configurationOutcome(token, "rollback")
	if plan.Reboot {
		restore += "if [ -f " + quote(dir+"/restored_boot") + " ] && [ \"$(cat /proc/sys/kernel/random/boot_id)\" != \"$(cat " + quote(dir+"/restored_boot") + ")\" ]; then\n" + configurationCleanup(token, "rollback") + "exit 0\nfi\n"
	}
	restore += "for package in " + configurationPackages + "; do\ncp " + quote(dir) + "/\"$package\" /etc/config/\"$package\"\nuci -q revert \"$package\"\ndone\n"
	if plan.ServiceSnapshot {
		restore += "if [ -f " + quote(dir) + "/packages.extra ]; then\nwhile read package; do cp " + quote(dir) + "/\"$package\" /etc/config/\"$package\"; uci -q revert \"$package\"; done < " + quote(dir) + "/packages.extra\nfi\n"
	}
	restore += configurationReload(plan)
	if plan.Cloud {
		restore += cloudRestoreScript()
	}
	if plan.Reboot {
		restore += "cat /proc/sys/kernel/random/boot_id > " + quote(dir+"/restored_boot") + "\nsync\nreboot\n"
	} else {
		restore += configurationCleanup(token, "rollback")
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
  touch started
  sync
  if ! /bin/sh request; then ln recover decision 2>/dev/null || true; fi
 else
  ` + configurationCleanup(token, "cancel") + `exit 0
 fi
else
 if [ ! -f started ]; then
  ` + configurationCleanup(token, "cancel") + `exit 0
 fi
 if [ -f awaiting_boot ]; then printf '%s' "$current_boot" > ready; else ln recover decision 2>/dev/null || true; fi
fi
remaining=300
while [ ! -f decision ] && [ "$remaining" -gt 0 ]; do sleep 1; remaining=$((remaining-1)); done
ln recover decision 2>/dev/null || true
sync
case "$(cat decision)" in
 confirm)
 ` + configurationCleanup(token, "confirm") + ` ;;
 rollback)
 attempts=0
 while ! /bin/sh restore; do
  attempts=$((attempts+1))
  if [ "$attempts" -ge 60 ]; then printf '%s' "$attempts" > stuck; sync; exit 75; fi
  sleep 5
 done
 ;;
 *) exit 75 ;;
esac
`

	service := "#!/bin/sh /etc/rc.common\nSTART=99\nstart() {\nset -eu\numask 077\nmkdir " + quote(configurationLock) + " 2>/dev/null || test \"$(cat " + quote(configurationLock+"/token") + ")\" = " + quote(token) + " || return 73\nprintf '%s' " + quote(token) + " > " + quote(configurationLock+"/token") + "\nif [ ! -d " + quote(dir) + " ]; then\nmkdir " + quote(configurationLock+"-actor-"+token) + " 2>/dev/null || return 73\n" + configurationCleanup(token, "") + "return 0\nfi\nrm -f " + quote(dir+"/pid") + "\nstart-stop-daemon -S -b -m -p " + quote(dir+"/pid") + " -x /bin/sh -- " + quote(dir+"/rollback") + "\n}\n"
	request := "#!/bin/sh\ntrap '' HUP\nset -eu\ncd " + quote(dir) + "\nexec > apply.log 2>&1\n" + configurationLua(config, true, secrets...) + "fw3 -q print >/dev/null\nfor package in " + configurationPackages + cloudCommitPackages(plan) + "; do uci commit \"$package\"; done\n" + configurationReload(plan) + cloudRequest(plan)
	if plan.Reboot {
		request += "touch awaiting_boot\nsync\nreboot\n"
	} else {
		request += "cat /proc/sys/kernel/random/boot_id > ready\n"
	}

	script := preflight + `pending=$(uci changes)
test -z "$pending" || exit 74
command -v lua >/dev/null
command -v start-stop-daemon >/dev/null
command -v fw3 >/dev/null
` + configurationDNSOwnership(config) + `test ! -e ` + quote(recoveryService) + ` && test ! -L ` + quote(recoveryService) + ` || exit 73
umask 077
mkdir ` + quote(configurationLock) + ` 2>/dev/null || exit 73
printf '%s' ` + quote(token) + ` > ` + quote(configurationLock+"/token") + `
transferred=0
prepared=0
trap 'if [ "$transferred" = 0 ]; then if [ "$prepared" = 1 ]; then ` + recoveryService + ` disable 2>/dev/null || true; rm -f ` + quote(recoveryService) + `; rm -rf ` + quote(dir) + `; fi; rm -f ` + quote(configurationLock+"/token") + `; rmdir ` + quote(configurationLock) + `; fi' EXIT
mkdir ` + quote(dir) + ` 2>/dev/null || exit 73
prepared=1
cd ` + quote(dir) + `
` + serviceSnapshot + `printf '%s' ` + quote(token) + ` > ` + quote(dir+"/token") + `
printf confirm > ` + quote(dir+"/confirm") + `
printf rollback > ` + quote(dir+"/recover") + `
cat /proc/sys/kernel/random/boot_id > ` + quote(dir+"/boot") + `
for package in ` + configurationPackages + `; do cp /etc/config/"$package" ` + quote(dir) + `/"$package"; done
` + servicePackages + `printf '%s' ` + quote(restore) + ` > ` + quote(dir+"/restore") + `
printf '%s' ` + quote(watchdog) + ` > ` + quote(dir+"/rollback") + `
cat > ` + quote(dir+"/request") + ` <<'GLINET_REQUEST'
` + request + `GLINET_REQUEST
printf '%s' ` + quote(service) + ` > ` + quote(recoveryService) + `
chmod 700 ` + quote(recoveryService) + `
` + recoveryService + ` enable
sync
transferred=1
start-stop-daemon -S -b -m -p ` + quote(dir+"/pid") + ` -x /bin/sh -- ` + quote(dir+"/rollback") + `
for attempt in 1 2 3 4 5; do test ! -f armed || break; sleep 1; done
test -f armed || exit 76
touch activate
`

	return script
}

func configurationConfirmScript(token string, config Configuration, secrets ...map[string]string) string {
	return configurationConfirmScriptWithPlan(token, config, reloadPlan{}, secrets...)
}

func configurationConfirmScriptWithPlan(token string, config Configuration, plan reloadPlan, secrets ...map[string]string) string {
	result := resultDir + "/" + token
	ack := "if [ -f " + quote(result) + " ]; then\ncase \"$(cat " + quote(result) + ")\" in\nconfirm)\nif [ ! -d " + quote(configurationLock+"-actor-"+token) + " ] && [ ! -d " + quote(transactionDir+"-retired-"+token) + " ] && [ \"$(cat " + quote(transactionDir+"/token") + " 2>/dev/null || true)\" != " + quote(token) + " ] && ! grep -F " + quote(token) + " " + quote(recoveryService) + " >/dev/null 2>&1; then exit 0; fi\n;;\nrollback|cancel) exit 77 ;;\n*) exit 75 ;;\nesac\nfi\n"
	script := preflight + ack + "cd " + quote(transactionDir) + "\n" + "test -d " + quote(configurationLock) + " || exit 75\ntest \"$(cat ready)\" = \"$(cat /proc/sys/kernel/random/boot_id)\" || exit 75\ntest \"$(cat token)\" = " + quote(token) + " || exit 75\nfw3 -q print >/dev/null\n"
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
	if len(secrets) > 0 {
		for key, value := range secrets[0] {
			parts := strings.SplitN(key, ".", 3)
			if len(parts) != 3 {
				continue
			}
			script += "test \"$(uci -q get " + quote(parts[0]+"."+parts[1]+"."+parts[2]) + ")\" = " + quote(value) + " || exit 75\n"
		}
	}
	if plan.Cloud {
		script += cloudVerifyScript(plan.CloudEnabled)
	}
	return script + "ln confirm decision 2>/dev/null || test \"$(cat decision)\" = confirm || exit 75\nsync\nfor attempt in 1 2 3 4 5 6 7 8 9 10; do\n" + ack + "sleep 1\ndone\nexit 75\n"
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

const dnsOwnershipCheck = `test "$(uci -q get adguardhome.config.enabled || true)$(uci -q get adguardhome.config.dns_enabled || true)" != '11' || exit 65
`

func configurationDNSOwnership(config Configuration) string {
	for _, s := range config {
		if s.Package == "gl-dns-v2" || (s.Package == "dhcp" && s.Type == "dnsmasq") {
			return dnsOwnershipCheck
		}
	}
	return ""
}
