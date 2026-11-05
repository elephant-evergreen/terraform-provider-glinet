# Terraform Provider for GL.iNet

A community Terraform provider for GL.iNet routers running stock firmware. Manage existing Wi-Fi AP settings or a coordinated configuration of networks, Wi-Fi, DHCP, firewall rules, stock DNS, and Ethernet port mapping.

Initial development targets the Flint 2 (GL-MT6000) with firmware 4.11.0 in router mode. Resource names and behavior will follow the underlying OpenWrt configuration model.

## Status

This project is in initial development. No provider release is available. Hardware verification status and outstanding release requirements are documented in the pull request. The intended Terraform source address is `elephant-evergreen/glinet`.

The [`glinet_wireless_interface_settings` resource](docs/resources/wireless_interface_settings.md) manages an existing named OpenWrt `wifi-iface` section. Removing this resource from Terraform leaves the Wi-Fi interface and its settings intact. For coordinated changes, use [`glinet_uci_configuration`](docs/resources/uci_configuration.md). It supports named section creation, selected anonymous sections, firewall rule ordering, and write-only Wi-Fi keys.

## Try the development build

Follow the [development setup](docs/development.md) to build and use a local provider binary. There is no Registry installation command until a release has been published.

Changes can disconnect clients. Coordinated network or wireless changes reboot the router and use a persistent 300-second recovery service. See the [configuration guide](docs/configuration.md) for adoption, rotation, recovery, and state handling. Individual Wi-Fi settings changes reload Wi-Fi. Use a dedicated test router and a wired management connection. Before an update, the provider prepares a private recovery snapshot on the router. Unconfirmed changes are scheduled to restore the previous wireless configuration after 120 seconds. This recovery mechanism has fixture coverage; behavior on hardware, power loss, and router reboot has not been verified. It is not a replacement for an external backup.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) before proposing changes. Documentation, examples, diagnostics, issues, and pull requests use English.

Never include router credentials, Wi-Fi passwords, private keys, Terraform state, or configuration exports in an issue or pull request.

## License

Source code is licensed under the [Mozilla Public License 2.0](LICENSE). See [NOTICE](NOTICE) for copyright and third-party notices.

This project is not affiliated with or endorsed by GL.iNet or HashiCorp.
