# Development setup

Install [mise](https://mise.jdx.dev/getting-started.html) and run these commands from the repository root:

```sh
mise install
bin/check-for-agent
mise run docs
mkdir -p work/bin work/example
mise exec -- go build -o work/bin/terraform-provider-glinet .
cp examples/provider/provider.tf work/example/
cp examples/resources/glinet_wireless_interface/resource.tf work/example/
```

Create a local Terraform CLI configuration that points to this build:

```sh
cat > work/dev.tfrc <<EOF
provider_installation {
  dev_overrides {
    "elephant-evergreen/glinet" = "$PWD/work/bin"
  }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="$PWD/work/dev.tfrc"
```

Set `GLINET_HOST`, `GLINET_PRIVATE_KEY_FILE`, and `GLINET_KNOWN_HOSTS_FILE` in your shell. `GLINET_PORT` is optional and defaults to 22. The provider connects as root using an unencrypted OpenSSH private key and always verifies the router host key. Verify and register the router host keys through a trusted channel before connecting. Go SSH may negotiate a different host-key algorithm from OpenSSH; the corresponding key must also be verified and present in the selected known_hosts file. Never bypass a key mismatch or trust an unauthenticated key scan. Do not put key contents or credentials in HCL or shell history.

Edit the copied resource example to select an existing named AP section and the desired settings on a dedicated test router. A section is the UCI section name, such as `guest2g`; it is not an SSID or a radio name. Anonymous selectors such as `@wifi-iface[0]` are unsupported.

```sh
mise exec -- terraform -chdir=work/example plan
```

Terraform's development override uses the local binary. With this standalone example, no provider installation from the Registry is needed. Do not run `terraform init` to try to download this unreleased provider.

After an apply, confirm client connectivity independently. Do not edit the router through another tool during an apply. Manage each UCI section with one Terraform resource only. An unconfirmed change keeps its recovery snapshot and lock until recovery finishes; inspect router connectivity before retrying. Recovery snapshots are temporary, contain the previous wireless configuration, and stay on the router.

Destroy releases Terraform management and does not restore earlier settings or delete the Wi-Fi interface. Import an existing section using the example in the [wireless interface reference](resources/wireless_interface.md).

For coordinated network configuration, use the [configuration guide](configuration.md) and its synthetic example instead. Keep section ownership separate from the existing-AP resource.

## Scope and verification

The provider checks for GL.iNet GL-MT6000, firmware 4.11.0 and router mode. The existing-AP resource additionally requires an existing named AP. Coordinated configuration validates each managed section and rejects ambiguous identities before mutation. It uses existing `uci`, `ubus`, `jsonfilter`, `wifi`, and `start-stop-daemon` commands on the router and does not install packages.

Local tests cover Terraform protocol behavior, import, refresh, drift, forgetting management, SSH host-key rejection, cancellation, bounded output, quoting, and recovery with a local command fixture. A read-only import plan has also been exercised on the target firmware. An explicitly authorized native UCI fixture also checks isolated temporary configuration on the supported firmware, including ordering, identity repair, lists and key handling; ordinary CI skips that gated test. These checks do not establish live network activation, radio behavior, compatibility with other firmware, or physical power-loss/reboot recovery. Hardware acceptance testing is required before publishing a usable release.

## Releases and licenses

The source is MPL-2.0. Preserve the license and copyright notices of dependencies when distributing a binary. Do not publish release tags or claim Registry availability until hardware acceptance testing, independent review, signed release artifacts, and the Registry publication setup are complete.
