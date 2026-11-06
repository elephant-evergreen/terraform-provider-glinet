# Terraform Provider for GL.iNet

A community Terraform provider for GL.iNet routers running stock firmware. Manage independent network, Wi-Fi, DHCP, firewall, stock DNS, Ethernet port, and WireGuard objects through typed Terraform resources.

Initial development targets the Flint 2 (GL-MT6000) with firmware 4.11.0 in router mode. Resource names and behavior will follow the underlying OpenWrt configuration model.

## Status

This project is in initial development. No provider release is available. Hardware verification is documented in the repository; WireGuard and OpenVPN remain hardware-unverified. The intended Terraform source address is `elephant-evergreen/glinet`.

The supported API is the typed resource set described in [typed resources](docs/typed-resources.md): network interfaces, bridges, Ethernet ports, Wi-Fi interfaces, DHCP pools/reservations, stock DNS, dnsmasq, firewall zones/forwardings/rules/redirects, experimental WireGuard interfaces/peers, plus the `glinet_system` data source. OpenVPN remains planned and is not shipped. Each resource owns one UCI object, supports import by stable identity, detects drift on refresh, and uses write-only arguments for secrets.

## Try the development build

Follow the [development setup](docs/development.md) to build and use a local provider binary. There is no Registry installation command until a release has been published.

Changes can disconnect clients. Network, Ethernet port-mapping, and wireless topology changes reboot the router and use the existing persistent 300-second recovery service. Every typed write snapshots the six supported UCI packages and is confirmed by the running firewall check. See [typed resources](docs/typed-resources.md). Individual Wi-Fi settings changes use the same serialized recovery worker and reload Wi-Fi. Use a dedicated test router and a wired management connection. Before an update, the provider prepares a private recovery snapshot on the router. Unconfirmed changes are scheduled to restore the previous configuration after the shared 300-second confirmation window. This recovery mechanism has fixture coverage; behavior on hardware, power loss, and router reboot has not been verified. It is not a replacement for an external backup.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) before proposing changes. Contributions in any language are welcome.

Never include router credentials, Wi-Fi passwords, private keys, Terraform state, or configuration exports in an issue or pull request.

## License

Source code is licensed under the [Mozilla Public License 2.0](LICENSE). See [NOTICE](NOTICE) for copyright and third-party notices.

This project is not affiliated with or endorsed by GL.iNet or HashiCorp.
