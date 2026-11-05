# Design and dependencies

The provider manages existing OpenWrt AP settings through a small typed resource. SSID, disabled, hidden, and isolate map to their UCI meanings. Section identity is the existing named wireless section, so no additional generated ID is needed. Destroy forgets management. The resource does not own interface creation, passwords, network topology, or firewall policy.

SSH is the transport because the initial GL.iNet target already exposes SSH and UCI. A read-only capability probe on the target found no `uci` or `session` ubus objects. The upstream rpcd apply/confirm transaction therefore cannot be assumed available. The implementation keeps a private wireless snapshot on the router, arms a separate recovery process before writing, serializes changes, refuses outstanding UCI changes, and confirms only its own completed change after reconnecting and reading settings back. Confirmation and recovery claim the same decision directory atomically. Recovery does not survive a router reboot.

No Wi-Fi password or complete router export is read into Terraform. Diagnostics discard remote stderr and do not include rejected command output. SSH host-key verification cannot be disabled. SSH keys are read from files, not stored in the Terraform schema.

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
