# Compatibility

| Item | Supported |
| --- | --- |
| Router | GL.iNet Flint 2 (GL-MT6000) |
| Firmware | Stock 4.11.x in router mode; other minors are rejected |
| Terraform | Resources and data sources; actions require Terraform 1.14 or newer |
| OpenTofu | Resources and data sources; provider actions are not supported |
| Provider OS | Linux and macOS on amd64 and arm64 |
| SSH | OpenSSH key authentication as root; verified known-hosts entry required; host-key checking cannot be disabled |
| Backups | age-encrypted full configuration archive; the SSH private key decrypts it |

Firmware 4.11.x is verified as a line. `mise run acceptance` runs the automated hardware acceptance against a dedicated router. Traffic acceptance across each network and power-loss recovery remain separate manual checks. See [verification boundaries](testing.md) and the [Admin Panel coverage matrix](coverage.md).

WireGuard resources are experimental.
