# Admin Panel coverage

This matrix defines the provider's coverage target for the supported router: GL.iNet Flint 2 (GL-MT6000), stock firmware 4.11.x, router mode. It follows the [GL.iNet 4.x Interface Guide](https://docs.gl-inet.com/router/en/4/interface_guide/).

Every row is a terminal state: `Managed` when the stock-exposed declarative settings are owned by a resource, `Out of scope` when the stock firmware publishes no field, file, or runtime state that Terraform can own, `Action` for one-shot operations, and `Read-only` for observation.

## Status legend

| Status | Meaning |
| --- | --- |
| Managed | The stock-exposed declarative settings are represented by a resource or attribute and covered by provider tests and docs. |
| Action | One-shot operation exposed as a Terraform action. |
| Read-only | Observation only; exposed or intended as a data source. |
| Out of scope | No Terraform-ownable surface: unpublished field schema, account-bound activation, file-based configuration, or runtime state. The reason is recorded in the row. |

## Completion rule

A row is complete when its status and the managed resources are accurate and the referenced tests pass. The provider does not claim coverage for a field the stock firmware does not publish; those rows are recorded as out of scope with the concrete reason.

Hardware verification is tracked separately: the release notes state which firmware build passed the acceptance run, and no release is prepared before that run.

## Network

| Admin Panel setting | Provider | Status |
| --- | --- | --- |
| Ethernet WAN/LAN roles and port mapping | `glinet_ethernet_port`, `glinet_network_interface`, `glinet_network_bridge` | Managed |
| WAN protocol: DHCP, static, PPPoE | `glinet_network_interface` | Managed: write-only credentials |
| LAN address and subnet | `glinet_network_interface`, `glinet_network_bridge`, `glinet_dhcp_pool` | Managed |
| Guest network | `glinet_network_interface`, `glinet_dhcp_pool`, `glinet_wireless_interface`, `glinet_firewall_zone` | Managed |
| IoT network | `glinet_network_interface`, `glinet_dhcp_pool`, `glinet_wireless_interface`, `glinet_firewall_zone` | Managed |
| DNS | `glinet_dns`, `glinet_dnsmasq`, `glinet_adguard` | Managed |
| Ethernet port and VLAN | `glinet_ethernet_port`, `glinet_network_interface` | Managed |
| IPv6 | `glinet_network_interface`, `glinet_network_route6`, `glinet_dhcp_pool` | Managed |
| MAC address clone | `glinet_network_interface` (`macaddr`), `glinet_network_bridge` (`macaddr`) | Managed |
| IGMP snooping | `glinet_network_bridge` | Managed |
| Network mode | `glinet_network_mode` | Managed |
| Drop-in gateway | `glinet_dropin_gateway`, `glinet_dropin_dhcp` | Managed |
| Hardware/network acceleration | `glinet_hardware_acceleration`, `glinet_firewall_defaults` | Managed |
| NAT settings | `glinet_nat`, `glinet_firewall_defaults` | Managed |
| Static routes | `glinet_network_route` | Managed |
| Policy routing | `glinet_network_rule`, `glinet_route_policy`, `glinet_route_policy_rule` | Managed |
| Multi-WAN, failover, load balancing | `glinet_kmwan`, `glinet_kmwan_member` | Managed |
| Repeater/WISP | `glinet_repeater` | Managed: joining an uplink stays a stock flow |
| USB tethering | None | Out of scope: runtime state that depends on an attached modem |
| Cellular | None | Out of scope: GL-MT6000 has no integrated cellular modem |

## Wireless

| Admin Panel setting | Provider | Status |
| --- | --- | --- |
| SSID, security, hidden SSID, client isolation | `glinet_wireless_interface` | Managed |
| Radio channel, width, transmit power, country | `glinet_wireless_radio` | Managed |
| Guest/IoT Wi-Fi | `glinet_wireless_interface`, `glinet_network_interface`, `glinet_dhcp_pool` | Managed |
| Wireless schedule | `glinet_timer`, `glinet_wifi_timer` | Managed |
| MAC filter/access control | `glinet_client_filter` | Managed: filter mode |
| MAC filter address list | None | Out of scope: the stock firmware publishes no option name for list entries |
| Mesh | `glinet_mesh` | Managed: pairing stays a stock flow |
| Client list | `glinet_clients` data source | Read-only |
| Block client | `glinet_client_filter` | Managed: filter mode |
| EAP/enterprise Wi-Fi | None | Out of scope: this stock build exposes no enterprise Wi-Fi settings |
| Wi-Fi 6/radio advanced options | None | Out of scope: the stock firmware publishes no additional radio field beyond the managed ones |

## Firewall and security

| Admin Panel setting | Provider | Status |
| --- | --- | --- |
| Zones, forwarding, redirects, filter rules | `glinet_firewall_zone`, `glinet_firewall_forwarding`, `glinet_firewall_redirect`, `glinet_firewall_rules` | Managed |
| Port forwarding | `glinet_firewall_redirect` | Managed |
| Firewall defaults | `glinet_firewall_defaults` | Managed |
| Stock firewall includes | None | Out of scope: the provider refuses to replace the firmware include scripts |
| NAT/UPnP | `glinet_nat`, `glinet_upnp` | Managed |
| ACL | `glinet_wan_access` | Managed: WAN whitelist switch |
| Admin access restrictions | `glinet_admin_access` | Managed: listeners and limits; credentials stay outside Terraform |
| Security scan | None | Out of scope: vendor cloud scan |
| Content filter | `glinet_content_protection` | Managed |
| Parental control | `glinet_parental_control` | Managed: global settings |
| Parental control rules | None | Out of scope: the stock firmware publishes no rule field schema |

## VPN

| Admin Panel setting | Provider | Status |
| --- | --- | --- |
| WireGuard interface and peer primitives | `glinet_wireguard_interface`, `glinet_wireguard_peer` | Managed: experimental |
| WireGuard server profile | `glinet_wireguard_server` | Managed |
| WireGuard generated client profiles | None | Out of scope: the stock UI generates and distributes client configuration |
| OpenVPN client | `glinet_openvpn` | Managed: custom configuration switch |
| OpenVPN server | `glinet_openvpn_server` | Managed |
| VPN dashboard, policy, kill switch | `glinet_wireguard_global`, `glinet_route_policy`, `glinet_route_policy_rule` | Managed |
| ExpressVPN and provider profiles | None | Out of scope: account-bound provider activation |
| Tailscale | `glinet_tailscale` | Managed |
| ZeroTier | `glinet_zerotier` | Managed |
| Tor | `glinet_tor` | Managed |

## Cloud services

| Admin Panel setting | Provider | Status |
| --- | --- | --- |
| GoodCloud switch | `glinet_goodcloud` | Managed |
| GL.iNet account binding | None | Out of scope: account-bound activation |
| GoodPAS | None | Out of scope: account-bound activation |
| AstroWarp | None | Out of scope: account-bound activation |
| Site to site | `glinet_s2s` | Managed |

## Flow control

| Admin Panel setting | Provider | Status |
| --- | --- | --- |
| QoS | `glinet_qos`, `glinet_qos_interface` | Managed |
| SQM | `glinet_sqm`, `glinet_sqm_service` | Managed |
| DPI engine | `glinet_dpi` | Managed |
| Content filter | `glinet_content_protection` | Managed |
| Parental control | `glinet_parental_control` | Managed |
| Network quality | None | Read-only: runtime measurement |
| Data statistics | None | Read-only: runtime counters |

## Applications

| Admin Panel setting | Provider | Status |
| --- | --- | --- |
| AdGuard Home | `glinet_adguard` | Managed: integration switch |
| AdGuard Home YAML configuration | None | Out of scope: the file is not UCI and the integration owns it |
| Dynamic DNS | `glinet_ddns` | Managed: service ports |
| DDNS provider credentials | None | Out of scope: account-bound credentials |
| Network storage/Samba/DLNA | `glinet_samba`, `glinet_minidlna` | Managed: global settings |
| Samba share sections | None | Out of scope: the stock firmware publishes no share option schema |
| Plug-ins/package installation | None | Out of scope: package installation is outside declarative router configuration |
| Scheduled tasks | `glinet_wifi_timer`, `glinet_led_timer`, `glinet_scheduled_reboot` | Managed: the stock schedules |

## System

| Admin Panel setting | Provider | Status |
| --- | --- | --- |
| Model, firmware, mode | `glinet_system` | Read-only |
| Hostname and time zone | `glinet_system_settings` | Managed |
| NTP | `glinet_system_ntp` | Managed |
| Logging | `glinet_system_settings` | Managed |
| SSH access | `glinet_system_ssh` | Managed |
| LED control | `glinet_led`, `glinet_system_led` | Managed |
| Admin password and users | None | Out of scope: credentials stay outside Terraform |
| Reboot | `glinet_reboot` action | Action |
| Backup | `glinet_backup` action | Action |
| Restore | None | Out of scope: destructive firmware-level operation left to the stock UI |
| Firmware upgrade | None | Out of scope: destructive firmware-level operation left to the stock UI |
| Scheduled reboot | `glinet_scheduled_reboot` | Managed |

## Current provider map

The matrix is backed by these resources and data sources:

- `glinet_network_interface`, `glinet_network_bridge`, `glinet_network_route`, `glinet_network_route6`, `glinet_network_rule`, `glinet_network_mode`
- `glinet_ethernet_port`, `glinet_wireless_interface`, `glinet_wireless_radio`, `glinet_mesh`, `glinet_repeater`
- `glinet_dhcp_pool`, `glinet_dhcp_reservation`, `glinet_dns`, `glinet_dnsmasq`, `glinet_ddns`, `glinet_adguard`
- `glinet_firewall_zone`, `glinet_firewall_forwarding`, `glinet_firewall_rules`, `glinet_firewall_redirect`, `glinet_firewall_defaults`
- `glinet_kmwan`, `glinet_kmwan_member`, `glinet_route_policy`, `glinet_route_policy_rule`, `glinet_dropin_gateway`, `glinet_dropin_dhcp`
- `glinet_client_filter`, `glinet_wan_access`, `glinet_parental_control`, `glinet_content_protection`, `glinet_dpi`
- `glinet_wireguard_interface`, `glinet_wireguard_peer`, `glinet_wireguard_server`, `glinet_wireguard_global`
- `glinet_openvpn`, `glinet_openvpn_server`, `glinet_tailscale`, `glinet_zerotier`, `glinet_tor`, `glinet_s2s`, `glinet_goodcloud`
- `glinet_qos`, `glinet_qos_interface`, `glinet_sqm`, `glinet_sqm_service`, `glinet_nat`, `glinet_hardware_acceleration`
- `glinet_upnp`, `glinet_samba`, `glinet_minidlna`, `glinet_admin_access`
- `glinet_system_settings`, `glinet_system_ntp`, `glinet_system_ssh`, `glinet_system_led`, `glinet_led`
- `glinet_timer`, `glinet_wifi_timer`, `glinet_led_timer`, `glinet_scheduled_reboot`
- `glinet_system` and `glinet_clients` data sources
- `glinet_reboot` and `glinet_backup` actions
