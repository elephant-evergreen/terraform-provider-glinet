# Terraform Provider for GL.iNet

Manage network interfaces, Wi-Fi, DHCP, DNS, and firewall settings on GL.iNet Flint 2 (GL-MT6000) running stock firmware 4.11.0 in router mode. Start with the [Terraform Registry reference](https://registry.terraform.io/providers/elephant-evergreen/glinet/latest/docs) or check the [release notes](https://github.com/elephant-evergreen/terraform-provider-glinet/releases) before changing versions.

## Check the router

Set `GLINET_HOST`, `GLINET_PRIVATE_KEY_FILE`, and `GLINET_KNOWN_HOSTS_FILE`. Generate or use an existing Ed25519 or RSA SSH key authorized for root, and verify the router's host key through a trusted channel before adding it to your known-hosts file.

Save the following as `main.tf` in an empty directory, then run `terraform init` and `terraform plan`. The plan reads the router model, firmware, and operating mode over SSH without changing configuration.

```hcl
terraform {
  required_providers {
    glinet = {
      source  = "elephant-evergreen/glinet"
      version = "~> 0.1.0"
    }
  }
}

provider "glinet" {}

data "glinet_system" "router" {}

output "firmware" {
  value = data.glinet_system.router.firmware
}
```

Keep `.terraform.lock.hcl` in version control. To pick up a compatible patch, read its release notes, run `terraform init -upgrade`, and review the lockfile and plan.

## Adopt configuration

Import existing objects before changing their settings and review the resulting plan. The provider verifies an age-encrypted full configuration backup before the first change in an apply. Network, port, and Wi-Fi changes also require an active wired management path. Router-side recovery rolls back unconfirmed changes after 300 seconds, including after an SSH disconnection or router reboot. See [typed resources and import IDs](docs/typed-resources.md) and the [scope of hardware testing](docs/testing.md). WireGuard remains experimental.

## Development

See [development](docs/development.md), [contributing](CONTRIBUTING.md), and the [release procedure](docs/releases.md).

## License

[MPL-2.0](LICENSE). Third-party notices are in [NOTICE](NOTICE). This project is not affiliated with or endorsed by GL.iNet or HashiCorp.
