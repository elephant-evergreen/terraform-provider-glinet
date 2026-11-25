# Terraform Provider for GL.iNet

Manage network interfaces, Wi-Fi, DHCP, DNS, firewall, and GoodCloud settings on GL.iNet Flint 2 (GL-MT6000) running stock firmware 4.11.x in router mode. Firmware 4.11.0 is the verified build; stable 4.11.x patches are allowed with a warning until they pass the hardware checks. See the [Terraform Registry reference](https://registry.terraform.io/providers/elephant-evergreen/glinet/latest/docs) or check the [release notes](https://github.com/elephant-evergreen/terraform-provider-glinet/releases) before changing versions.

## Check the router

Follow [Check the router in Terraform Registry](https://registry.terraform.io/providers/elephant-evergreen/glinet/latest/docs#check-the-router) to verify SSH access and read the model, firmware, and operating mode.

## Adopt configuration

Open **Import existing configuration** on the Registry page for the resource you want to adopt: add the import block, match the configuration to the router's current values, and apply a plan that contains only the import.

The provider verifies an age-encrypted full configuration backup before the first change in an apply. Network, port, Wi-Fi, and GoodCloud changes also require an active wired management path. Router-side recovery rolls back unconfirmed changes after 300 seconds, including after an SSH disconnection or router reboot. See [typed resources and import IDs](docs/typed-resources.md) and the [scope of hardware testing](docs/testing.md). WireGuard remains experimental.

## GoodCloud

`glinet_goodcloud` calls the router's stock `cloud.set_config` operation. Disabling GoodCloud stops the stock `rtty` service, may stop `mptun` and `mpifd`, removes the local `bindtime`, may set `mptun.global.enable=0`, and may add a default `server` when none is configured. Enabling it starts the stock agent; a router without an account binding or server may not start a process. This resource does not bind or unbind the router in a GL.iNet account. Destroy releases provider ownership and leaves the current switch value unchanged. See the resource page for the exact contract and rollback limits.

## Development

See [development](docs/development.md), [contributing](CONTRIBUTING.md), and the [release procedure](docs/releases.md).

## License

[MPL-2.0](LICENSE). Third-party notices are in [NOTICE](NOTICE). This project is not affiliated with or endorsed by GL.iNet or HashiCorp.
