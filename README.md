# Terraform Provider for GL.iNet

Manage Flint 2 (GL-MT6000), stock firmware 4.11.0 in router mode through typed Terraform resources. Install [elephant-evergreen/glinet from Terraform Registry](https://registry.terraform.io/providers/elephant-evergreen/glinet/0.1.0).

```hcl
terraform {
  required_providers {
    glinet = {
      source  = "elephant-evergreen/glinet"
      version = "= 0.1.0"
    }
  }
}

provider "glinet" {}
```

```sh
terraform init
terraform plan
terraform apply
```

Provide `GLINET_HOST`, `GLINET_PRIVATE_KEY_FILE`, and `GLINET_KNOWN_HOSTS_FILE`, or set the corresponding provider arguments. Use an Ed25519 or RSA SSH private key for root and host keys verified through a trusted channel. Host-key verification cannot be disabled. Terraform installs versioned provider packages and verifies their checksums using its dependency lock file.

Before a mutation, the provider checks configuration ownership and conflicts, verifies an age-encrypted full router configuration backup, and requires a direct active wired management path for network, port, or Wi-Fi changes. Each resource uses a persistent router-side recovery transaction. Unconfirmed changes roll back after 300 seconds, including after SSH disconnection or a router reboot. External encrypted backups and transaction snapshots serve different recovery needs.

Resources own network interfaces, bridges, Ethernet ports, Wi-Fi interfaces, DHCP pools and reservations, DNS, dnsmasq, firewall zones, forwardings, rules and redirects. WireGuard resources remain experimental. Resources support import, refresh, and drift detection; secret inputs are write-only. See [typed resources](docs/typed-resources.md) and the [provider reference](docs/index.md).

## Development

The provider owns compilation, tests, documentation, and releases. See [development](docs/development.md), [verification boundaries](docs/testing.md), and [contributing](CONTRIBUTING.md).

## License

[MPL-2.0](LICENSE). Third-party notices are in [NOTICE](NOTICE). This project is not affiliated with or endorsed by GL.iNet or HashiCorp.
