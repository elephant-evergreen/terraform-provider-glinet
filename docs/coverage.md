# Admin Panel coverage

This matrix defines the provider's 100% target for the supported router: GL.iNet Flint 2 (GL-MT6000), stock firmware 4.11.x, router mode. It follows the [GL.iNet 4.x Interface Guide](https://docs.gl-inet.com/router/en/4/interface_guide/).

The target covers **declarative settings** in the Admin Panel. One-shot operations are tracked separately as actions. Runtime statistics and vendor-internal state are read-only or out of scope.

## Status legend

| Status | Meaning |
| --- | --- |
| Managed | The declarative setting is represented by a resource or attribute and is covered by provider tests and docs. |
| Partial | Some values are managed, but a material part of the Admin Panel setting is not. |
| Planned | No provider API exists yet. |
| Action | One-shot operation; it belongs in an action or data source, not a Terraform resource. |
| Read-only | Observation only; it belongs in a data source. |
| Out of scope | Not applicable to this model or intentionally excluded. |

## Completion rule

The provider may claim 100% coverage only when:

1. Every declarative row marked `Partial` or `Planned` is `Managed`.
2. Every `Action` row has a documented action or data source, or is explicitly excluded with a reason.
3. Every `Out of scope` row has a model, firmware, or runtime justification.
4. The supported hardware matrix has passed the release checks for the exact firmware.

This matrix is the source of truth for scope. A row is not complete merely because the firmware stores a value in UCI; it must also be safe to own, reversible, and testable.

## Network

| Admin Panel setting | Current provider | Status |
| --- | --- | --- |
| Ethernet WAN/LAN roles and port mapping | `glinet_ethernet_port`, `glinet_network_interface` | Partial: port roles and interfaces, but not every WAN mode |
| WAN protocol: DHCP, static, PPPoE | `glinet_network_interface` | Managed: DHCP/static/PPPoE with write-only credentials |
| LAN address and subnet | `glinet_network_interface`, `glinet_network_bridge` | Partial: address/subnet and bridge, not all DHCP/subnet options |
| Guest network | `glinet_network_interface`, `glinet_dhcp_pool`, `glinet_wireless_interface` | Partial: interfaces, pool, SSID; no dedicated guest-network resource |
| IoT network | `glinet_network_interface`, `glinet_dhcp_pool`, `glinet_wireless_interface` | Partial: interfaces, pool, SSID; no dedicated IoT resource |
| DNS | `glinet_dns`, `glinet_dnsmasq` | Partial: stock DNS mode/dnsmasq; AdGuard Home is separate |
| Ethernet port and VLAN | `glinet_ethernet_port`, `glinet_network_interface` | Managed |
| IPv6 | `glinet_network_interface`, `glinet_network_route6` | Partial: interface hints and routes; no full address/RA coverage |
| MAC address clone | `glinet_network_interface` (`macaddr`) | Managed |
| IGMP snooping | `glinet_network_bridge` | Managed |
| Network mode | `glinet_network_mode` | Managed |
| Drop-in gateway | `glinet_dropin_gateway` | Partial: global switch; helper DHCP section remains |
| Hardware/network acceleration | `glinet_hardware_acceleration`, `glinet_firewall_defaults` | Managed |
| NAT settings | `glinet_nat`, `glinet_firewall_defaults` | Managed |
| Static routes | `glinet_network_route` | Managed |
| Policy routing | `glinet_network_rule`, `glinet_route_policy` | Managed for the global policy; GL per-rule sections remain |
| Multi-WAN, failover, load balancing | `glinet_kmwan`, `glinet_kmwan_member` | Managed |
| Repeater/WISP | `glinet_repeater` | Partial: uplink policy; joining a network remains a stock flow |
| USB tethering | None | Out of scope: runtime state that depends on an attached modem |
| Cellular | None | Out of scope for GL-MT6000: no integrated cellular modem |

## Wireless

| Admin Panel setting | Current provider | Status |
| --- | --- | --- |
| SSID, security, hidden SSID, client isolation | `glinet_wireless_interface` | Managed |
| Radio channel, width, transmit power, country | `glinet_wireless_radio` | Managed |
| Guest/IoT Wi-Fi | `glinet_wireless_interface` | Partial: SSID-level only; no dedicated network workflow |
| Wireless schedule | `glinet_timer`, `glinet_wifi_timer` | Managed |
| MAC filter/access control | `glinet_client_filter` | Partial: filter mode; address list remains in the stock UI |
| Mesh | `glinet_mesh` | Partial: base settings; pairing remains a stock flow |
| Client list and block client | `glinet_client_filter` | Partial: MAC filter mode; live client list remains read-only |
| EAP/enterprise Wi-Fi | None | Out of scope: this stock build exposes no enterprise Wi-Fi settings |
| Wi-Fi 6/radio advanced options | None | Planned |

The 0.2.x line keeps the remaining OpenWrt encryption values (`psk`, `wpa`, `wpa2`, `wpa-mixed`, `owe`) so existing configurations continue to plan. The next compatible release adds `ieee80211w` and `wps_pushbutton` without changing existing defaults. The next minor release will restrict `encryption` to the five stock modes (`none`, `psk2`, `psk-mixed`, `sae`, `sae-mixed`) and default `wps_pushbutton` to disabled. The release before that restriction will warn in the plan and document the migration. Radio settings are covered by schema, fixture, and protocol tests; a representative channel or width change still needs the hardware acceptance run before the provider claims 100%.

## Firewall and security

| Admin Panel setting | Current provider | Status |
| --- | --- | --- |
| Zones, forwarding, redirects, filter rules | `glinet_firewall_zone`, `glinet_firewall_forwarding`, `glinet_firewall_redirect`, `glinet_firewall_rules` | Managed for the modeled options |
| Port forwarding | `glinet_firewall_redirect` | Managed |
| Firewall defaults and includes | `glinet_firewall_defaults` | Partial: defaults managed; stock includes remain protected |
| NAT/UPnP | `glinet_upnp` | Partial: UPnP managed; NAT defaults remain |
| ACL | `glinet_wan_access` | Partial: WAN whitelist switch |
| Admin access restrictions | `glinet_admin_access` | Partial: listeners and limits; credentials remain external |
| Security scan | None | Action |
| Content filter | `glinet_content_protection` | Managed |
| Parental control | `glinet_parental_control` | Partial: global settings; rules remain |

## VPN

| Admin Panel setting | Current provider | Status |
| --- | --- | --- |
| WireGuard interface and peer primitives | `glinet_wireguard_interface`, `glinet_wireguard_peer` | Partial: low-level UCI only, experimental and hardware-unverified |
| WireGuard client/server profiles | `glinet_wireguard_server`, `glinet_wireguard_interface`, `glinet_wireguard_peer` | Partial: server and primitives; generated client profiles remain in the stock UI |
| OpenVPN client | `glinet_openvpn` | Partial: custom config switch; profile file is not managed |
| OpenVPN server | `glinet_openvpn_server` | Partial: basic server options; certificates are paths only |
| VPN dashboard, policy, kill switch | `glinet_wireguard_global`, `glinet_route_policy` | Partial: global proxy and policy switch |
| ExpressVPN and provider profiles | None | Out of scope: account-bound provider activation |
| Tailscale | `glinet_tailscale` | Managed for settings |
| ZeroTier | `glinet_zerotier` | Managed for integration |
| Tor | `glinet_tor` | Managed for integration |

## Cloud services

| Admin Panel setting | Current provider | Status |
| --- | --- | --- |
| GoodCloud switch | `glinet_goodcloud` | Managed; unreleased and hardware-unverified |
| GL.iNet account binding | None | Out of scope: account-bound activation |
| GoodPAS | None | Out of scope: account-bound activation |
| AstroWarp | None | Out of scope: account-bound activation |

## Flow control

| Admin Panel setting | Current provider | Status |
| --- | --- | --- |
| QoS | `glinet_qos`, `glinet_qos_interface` | Managed |
| SQM | `glinet_sqm`, `glinet_sqm_service` | Managed |
| DPI engine | `glinet_dpi` | Managed |
| Content filter | `glinet_content_protection` | Managed |
| Parental control | `glinet_parental_control` | Partial: global settings; rules remain |
| Network quality | None | Read-only |
| Data statistics | None | Read-only |

## Applications

| Admin Panel setting | Current provider | Status |
| --- | --- | --- |
| AdGuard Home | `glinet_adguard` | Partial: integration switch; YAML config is not managed |
| Dynamic DNS | `glinet_ddns` | Partial: service ports; provider credentials remain external |
| Network storage/Samba/DLNA | `glinet_minidlna`, `glinet_samba` | Partial: DLNA and Samba globals; share sections remain |
| Plug-ins/package installation | None | Action |
| Scheduled tasks | `glinet_wifi_timer`, `glinet_led_timer` | Partial: Wi-Fi and LED schedules; arbitrary cron jobs remain |

## System

| Admin Panel setting | Current provider | Status |
| --- | --- | --- |
| Model, firmware, mode | `glinet_system` data source | Read-only |
| Time zone/NTP | `glinet_system_settings`, `glinet_system_ntp` | Managed |
| Admin password and users | None | Out of scope: credentials stay outside Terraform |
| SSH access | `glinet_system_ssh` | Managed |
| LED control | `glinet_system_led`, `glinet_led` | Managed |
| Logs and diagnostics | None | Read-only |
| Reboot | `glinet_reboot` action | Action implemented |
| Firmware upgrade | None | Action |
| Backup/restore | `glinet_backup` action | Partial: backup action; restore remains |
| Scheduled reboot | `glinet_scheduled_reboot` | Managed |

## Current provider map

The matrix is backed by these resources and data sources:

- `glinet_network_interface`
- `glinet_network_bridge`
- `glinet_network_route`
- `glinet_ethernet_port`
- `glinet_wireless_interface`
- `glinet_wireless_radio`
- `glinet_dhcp_pool`
- `glinet_dhcp_reservation`
- `glinet_dns`
- `glinet_dnsmasq`
- `glinet_firewall_zone`
- `glinet_firewall_forwarding`
- `glinet_firewall_rules`
- `glinet_firewall_redirect`
- `glinet_wireguard_interface`
- `glinet_wireguard_peer`
- `glinet_goodcloud`
- `glinet_system` (data source)

`glinet_firewall_rule` and `position` were removed in 0.2.1.

## Priority order

1. WAN/IPv6/routing and multi-WAN
2. Wireless radio and client controls
3. Firewall defaults, NAT, UPnP, ACL, admin access
4. VPN profiles and policy
5. Cloud and flow-control settings
6. Applications and system settings
7. Actions and read-only observations

Each family gets its own PR with schema, safety analysis, tests, docs, and release notes. A release is prepared only after the matrix has no remaining `Partial` or `Planned` declarative rows and the hardware verification is complete.
