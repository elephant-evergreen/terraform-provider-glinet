# Typed resources

This provider's API is a set of typed Terraform resources that map to OpenWrt UCI objects.

## Design principles

The design follows the [HashiCorp provider design principles](https://developer.hashicorp.com/terraform/plugin/best-practices/hashicorp-provider-design-principles):

- A resource represents one UCI object, or one firmware singleton. The Terraform schema uses typed attributes rather than a JSON document.
- Identity, import, refresh, update, and destroy are defined per resource. Anonymous stock sections are resolved by a stable unique option instead of by their mutable `@type[index]` name.
- Secrets use write-only arguments. They are never stored in plan or state.
- Every create, update, and delete is one recovery-protected transaction. The existing snapshot, watchdog, boot service, and rollback engine is reused.
- Terraform's dependency graph, not a provider-side collection resource, orders related objects. Use references (`glinet_network_interface.lan.name`, `glinet_firewall_zone.lan.name`) so the graph is explicit.
- A failed multi-resource apply leaves each completed resource in a valid committed state. It never leaves a half-written UCI package. Re-running apply converges; the retry boundary is Terraform's normal plan/apply.

## Resources

| Resource | UCI object | Identity / import ID | Destroy default |
| --- | --- | --- | --- |
| `glinet_network_interface` | `network` interface | interface name (`lan`, `vlan11`) | forget |
| `glinet_network_bridge` | `network` bridge device | bridge name (`br-lan`) | forget |
| `glinet_ethernet_port` | `eth_ports_config_map` port | port name (`lan2`) | forget |
| `glinet_wireless_interface` | `wireless` wifi-iface | section name (`wifi2g`) | forget |
| `glinet_dhcp_pool` | `dhcp` pool | section name (`lan`) | forget |
| `glinet_dhcp_reservation` | `dhcp` host | section name (`home_iac_admin_lease`) | forget |
| `glinet_dns` | `gl-dns-v2` DNS singleton | `dns` | forget |
| `glinet_dnsmasq` | `dhcp` dnsmasq singleton | `dnsmasq` | forget |
| `glinet_firewall_zone` | firewall zone | zone name (`lan`) | forget |
| `glinet_firewall_forwarding` | firewall forwarding | `src:dest` | forget |
| `glinet_firewall_rule` | firewall rule | unique rule name | forget |
| `glinet_firewall_redirect` | firewall redirect | unique redirect name | forget |
| `glinet_wireguard_interface` | `network` WireGuard interface | interface name (`wg0`) | forget |
| `glinet_wireguard_peer` | `network` `wireguard_<iface>` peer | `interface:section` | forget |
| `glinet_system` data source | router model, firmware, mode | n/a | n/a |

`delete_behavior = "delete"` removes a Terraform-created named section. Firmware-owned anonymous sections are always preserved, and `forget` releases the provider marker without deleting the object. Use `lifecycle { prevent_destroy = true }` on network and management objects.

## Transactions and consistency

Every typed write:

1. Reads the current marked configuration and the requested change.
2. Builds the full managed configuration and runs the existing preflight checks (bridge port ownership, unmanaged Wi-Fi/zone/forwarding/ACCEPT conflicts, DHCP conflicts, private LAN, IPv6, UPnP, include hashes, DNS provider metadata).
3. Acquires the provider mutex and the router transaction lock.
4. Snapshots the six UCI packages, stages the change, commits, reloads the affected service or reboots, and confirms the generated firewall.
5. On failure, the router-side watchdog restores the previous packages.

A marker (`home_iac_owner=1`) is written in the same transaction. It is how the provider distinguishes its own sections from firmware sections without a separate registry. Marker-only changes do not trigger a service reload or reboot.

Terraform has no cross-resource commit barrier. The provider therefore guarantees **per-resource atomicity**, **no partially written UCI package**, and **recovery on every mutation**. A multi-resource apply can stop after some resources have committed; each committed resource is valid, and a subsequent apply completes the rest. Use `depends_on` or attribute references for required ordering.

## Secrets

Wi-Fi keys use `key_wo` with `key_version`. The value is passed only in the provider process and the SSH transaction script, and is removed before state or plan is persisted. Named wifi-iface sections require 8–63 printable ASCII characters.

Reads never request secret options from UCI. The existing encrypted backup and recovery snapshot remain operator responsibilities and are not replaced by Terraform state.

## Import

Each resource exposes a stable import ID, listed in the table above. Anonymous stock sections are resolved by a stable unique option. Use `terraform import` or an `import` block, then review the generated configuration with `terraform plan -generate-config-out`.

The provider is the only owner of the sections it manages. Do not overlap typed resources with manual UCI edits or a second management tool.

## VPN status

WireGuard is implemented as an experimental typed resource family; OpenVPN remains specified. Neither is hardware-verified in this change. Do not use Python or shell fallbacks.

WireGuard resources:

- `glinet_wireguard_interface`: `name`, `addresses`, `listen_port`, `mtu`, `fwmark`, `no_host_route`, `enabled`, `private_key_wo`, `private_key_version`.
- `glinet_wireguard_peer`: `section`, `interface`, `public_key`, `allowed_ips`, `endpoint_host`, `endpoint_port`, `persistent_keepalive`, `route_allowed_ips`, `description`, `preshared_key_wo`, `preshared_key_version`.

The peer resolves the OpenWrt dynamic section type `wireguard_<interface>`. The private key and preshared key use the generic write-only secret transport (`package.section.option`), are checked during stable read, apply, and confirmation, and are redacted from safety-state reads.

OpenVPN remains planned:

| Planned resource | UCI object | Identity / import ID | Secrets |
| --- | --- | --- | --- |
| `glinet_openvpn_client` | `openvpn` client section | section name | `private_key_wo`, `auth_password_wo` |

Remaining before OpenVPN can be shipped: add the nonsecret option allowlist, import/mapping tests, secret names, service reload handling, generated docs, and hardware validation.

Do not manage VPN with external scripts.

## Hardware status

The typed resources have fixture, protocol, schema, and live plan/import coverage. The home-network configuration has been applied and verified on the target router. WireGuard and OpenVPN remain unverified on hardware; do not treat schema or fixture success as hardware verification for them.
