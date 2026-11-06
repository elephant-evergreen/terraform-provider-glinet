# Design and dependencies

The supported API is a typed resource set: one resource per network interface, bridge, Ethernet port, Wi-Fi interface, DHCP pool/reservation, stock DNS singleton, dnsmasq singleton, firewall zone/forwarding/rule/redirect, plus a read-only system data source. Each resource maps to a UCI object, uses a stable identity for import, and owns one recovery-protected transaction. A `home_iac_owner=1` marker written in the same transaction lets the preflight checks distinguish Terraform-managed sections from firmware sections.

SSH is the transport because the supported stock firmware exposes SSH and UCI but the capability probe found no uci or session ubus objects. The upstream rpcd apply/confirm transaction cannot be assumed available. Native libuci Lua APIs own section lookup, option/list mutation and ordering rather than a separate UCI implementation.

All resources use one persistent router worker. It snapshots all six configuration packages, synchronizes recovery preparation before activation, serializes mutations and restoration, and arbitrates confirmation versus recovery through an atomic durable decision. Network and wireless topology changes reboot; existing AP settings reload Wi-Fi. An init service resumes recovery or cleanup after reboot. Nonce-bound locks and boot-specific readiness reject stale participants. Small nonsecret terminal-outcome records make confirmation retryable after a lost SSH response; a rollback never acknowledges a successful apply. See [typed resources](typed-resources.md) for the recovery boundary and limitations.

Before mutation, the provider checks bridge port ownership, unmanaged Wi-Fi/zones/forwardings/redirects/ACCEPT rules, DHCP reservations, the private LAN address, IPv6, UPnP, stock include hashes, and stock DNS provider metadata. It also compares the generated IPv4 firewall with the running `iptables-save` during refresh and before confirmation. These checks stop on conflicts instead of overwriting policy that Terraform does not own.

Wi-Fi keys enter the wireless resource only through sensitive write-only arguments and an explicit generation. Refresh does not read keys or complete exports into Terraform. Private snapshots and requests remain on the router until cleanup. Diagnostics discard remote stderr and rejected-command output. Host-key verification cannot be disabled, and SSH credentials are read from files rather than Terraform state.

## Selected dependencies

| Dependency | Fixed version | Responsibility and rationale |
| --- | --- | --- |
| [Go](https://go.dev/dl/) | 1.27.3 | Official compiler and standard library; pinned with mise. |
| [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework/tree/v1.19.0) | v1.19.0 | HashiCorp's provider protocol and schema implementation; avoids implementing a Terraform runtime. MPL-2.0. |
| [Go SSH](https://pkg.go.dev/golang.org/x/crypto/ssh@v0.58.0) | v0.58.0 | Go project's SSH and known_hosts implementation; avoids implementing cryptography or disabling host verification. BSD-3-Clause. |
| [Terraform Plugin Testing](https://github.com/hashicorp/terraform-plugin-testing/tree/v1.16.0) | v1.16.0 | Development dependency for actual Terraform protocol, plan, state, refresh, and import tests. MPL-2.0. |
| [tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs/tree/v0.25.0) | v0.25.0 | Official schema-driven documentation generator. MPL-2.0. |

Go module versions and checksums are in go.mod and go.sum. Tests replace the external router transport or commands while using the shipping provider and Terraform Framework. Credentials are not available to ordinary CI. CI uses the official checkout and mise Actions pinned to reviewed commit IDs and a hosted Ubuntu runner with read-only repository access. Renovate proposes dependency updates without automatic merging.

The Framework's transitive gRPC dependency is pinned to v1.83.2 to address the Go vulnerability database findings GO-2026-6061, GO-2026-6348, and GO-2026-6443 affecting the upstream v1.79.3 selection. See the [official gRPC release](https://github.com/grpc/grpc-go/releases/tag/v1.83.2). Keep the override until the Framework selects a fixed version and verify updates with govulncheck and the protocol tests.

The upstream provider scaffold examples in [HashiCorp Agent Skills](https://github.com/hashicorp/agent-skills/tree/5e30fa5c867ba6777c37e4206babfd08049c396f) informed the provider entry point and schema layout. The implementation is original; the upstream template files are not copied into this repository.

Protocol references: [OpenWrt UCI](https://openwrt.org/docs/techref/uci), [rpcd transaction implementation](https://github.com/openwrt/rpcd/blob/469d7852316ef692805396d8af744ec43604b842/uci.c), [GL.iNet SSH access](https://docs.gl-inet.com/router/en/4/tutorials/ssh_log_in_to_the_router/), and [HashiCorp provider design principles](https://developer.hashicorp.com/terraform/plugin/best-practices/hashicorp-provider-design-principles).
