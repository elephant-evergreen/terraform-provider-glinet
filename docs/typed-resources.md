# Typed resources

This provider's supported API is a set of typed Terraform resources that map to OpenWrt UCI objects. `glinet_uci_configuration` is retained only for migration; it is not the target design.

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
| `glinet_system` data source | router model, firmware, mode | n/a | n/a |

`delete_behavior = "delete"` removes a Terraform-created named section. Firmware-owned anonymous sections are always preserved, and `forget` releases the provider marker without deleting the object. Use `lifecycle { prevent_destroy = true }` on network and management objects.

## Transactions and consistency

Every typed write:

1. Reads the current marked configuration and the requested change.
2. Builds the full managed configuration and runs the existing preflight checks (bridge port ownership, unmanaged Wi-Fi/zone/forwarding/ACCEPT conflicts, DHCP conflicts, private LAN, IPv6, UPnP, include hashes, DNS provider metadata).
3. Acquires the provider mutex and the router transaction lock.
4. Snapshots the six UCI packages, stages the change, commits, reloads the affected service or reboots, and confirms the generated firewall.
5. On failure, the router-side watchdog restores the previous packages.

A marker (`home_iac_owner=1`) is written in the same transaction. It is how the provider distinguishes its own sections from firmware sections without a separate registry. Marker-only changes do not trigger a service reload or reboot, so migrating an unchanged configuration is a no-op at the service level.

Terraform has no cross-resource commit barrier. The provider therefore guarantees **per-resource atomicity**, **no partially written UCI package**, and **recovery on every mutation**. A multi-resource apply can stop after some resources have committed; each committed resource is valid, and a subsequent apply completes the rest. Use `depends_on` or attribute references for required ordering.

## Secrets

Wi-Fi keys use `key_wo` with `key_version`. The value is passed only in the provider process and the SSH transaction script, and is removed before state or plan is persisted. `ValidWirelessKeys` still requires named wifi-iface sections and 8–63 printable ASCII characters.

Reads never request secret options from UCI. The existing encrypted backup and recovery snapshot remain operator responsibilities and are not replaced by Terraform state.

## Import and state migration

1. Keep the old configuration and state until every typed resource is imported.
2. Add typed resources with `import` blocks. The import IDs are in the table above. For stock sections, import by the stable `name`; the provider resolves the anonymous UCI section.
3. Run `terraform plan -generate-config-out=generated.tf` to generate HCL, then replace the import blocks with the generated configuration.
4. Remove `glinet_uci_configuration.home` from configuration and state after the typed resources are present:
   `terraform state rm glinet_uci_configuration.home`.
5. Run `terraform plan`. The expected diff is marker/attribute adoption only; no network reload should be required for marker-only changes.
6. Review and apply. Use an external encrypted backup before the first apply.

The old resource's destroy behavior remains "forget"; it never deletes router settings.

## VPN specification (not shipped)

WireGuard and OpenVPN are the next resource family. They are specified here so implementations stay inside the provider instead of falling back to Python or shell.

| Planned resource | UCI object | Identity / import ID | Secrets |
| --- | --- | --- | --- |
| `glinet_wireguard_interface` | `network` interface with `proto=wireguard` | interface name (`wg0`) | `private_key_wo` |
| `glinet_wireguard_peer` | `network` `wireguard` peer | `interface:public_key` | `preshared_key_wo` |
| `glinet_openvpn_client` | `openvpn` client section | section name | `private_key_wo`, `auth_password_wo` |

Planned attributes:

- WireGuard interface: `name`, `addresses` (set of CIDRs), `listen_port`, `mtu`, `fwmark`, `no_host_route`, `enabled`, `private_key_wo`, `private_key_version`.
- WireGuard peer: `interface`, `public_key`, `allowed_ips`, `endpoint_host`, `endpoint_port`, `persistent_keepalive`, `route_allowed_ips`, `description`, `preshared_key_wo`, `preshared_key_version`.
- OpenVPN client: `section`, `config`, `dev`, `proto`, `remote_host`, `remote_port`, `auth_user`, `auth_password_wo`, `private_key_wo`, `ca_file`, `cert_file`, `enabled`.

Implementation requirements before these resources are marked supported:

1. Add their nonsecret options and section types to the UCI allowlist.
2. Generalise write-only secret transport from wireless keys to arbitrary `package.section.option` values, with checks in stable read, apply, and confirmation.
3. Add mapper, schema, import, drift, delete, and fixture protocol tests.
4. Extend the transaction reload plan (network reload/reboot) and the full managed-config safety checks.
5. Generate resource documentation and add examples.
6. Verify on physical hardware; until then, mark the resources as unreleased/experimental and do not claim hardware support.

Do not manage VPN with the deprecated JSON resource or external scripts in a new configuration. The JSON resource is retained only for migration and compatibility.

## Hardware status

The typed resources are tested with fixture transports and `terraform validate`/protocol tests. They have not been applied to a physical router in this change. Do not treat plan or unit-test success as hardware verification.
