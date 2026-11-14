# Terraform Provider for GL.iNet

Manage network interfaces, Wi-Fi, DHCP, DNS, and firewall settings on GL.iNet Flint 2 (GL-MT6000) running stock firmware 4.11.0 in router mode. See the [Terraform Registry reference](https://registry.terraform.io/providers/elephant-evergreen/glinet/latest/docs) or check the [release notes](https://github.com/elephant-evergreen/terraform-provider-glinet/releases) before changing versions.

## Check the router

Follow [Check the router in Terraform Registry](https://registry.terraform.io/providers/elephant-evergreen/glinet/latest/docs#check-the-router) to verify SSH access and read the model, firmware, and operating mode.

## Adopt configuration

Open **Import existing configuration** on the Registry page for the resource you want to adopt. Run `terraform plan` and look for `1 to import, 0 to add, 0 to change, 0 to destroy` when adopting one object. Apply the import-only plan, then run `terraform plan` again and expect `No changes`.

The provider verifies an age-encrypted full configuration backup before the first change in an apply. Network, port, and Wi-Fi changes also require an active wired management path. Router-side recovery rolls back unconfirmed changes after 300 seconds, including after an SSH disconnection or router reboot. See [typed resources and import IDs](docs/typed-resources.md) and the [scope of hardware testing](docs/testing.md). WireGuard remains experimental.

## Development

See [development](docs/development.md), [contributing](CONTRIBUTING.md), and the [release procedure](docs/releases.md).

## License

[MPL-2.0](LICENSE). Third-party notices are in [NOTICE](NOTICE). This project is not affiliated with or endorsed by GL.iNet or HashiCorp.
