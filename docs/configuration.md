# Coordinated router configuration

Use one `glinet_uci_configuration` resource per router to manage network, wireless, DHCP, firewall, `gl-dns-v2`, and Ethernet port mapping together. Build the provider using the [development setup](development.md), then adapt the [synthetic example](../examples/resources/glinet_uci_configuration/resource.tf).

## Adopt existing settings

1. Read the router through a verified SSH connection and describe the desired managed sections using `jsonencode`. The provider requires the section type to match. Named sections are created if missing. Anonymous selectors such as `@zone[0]` select the unique section of that type; add `match = { name = "guest" }` when the type has multiple sections. The numeric selector is an identifier within the configuration; identity is resolved by type and the unique match rather than by mutable position.
2. Run `terraform plan` and inspect the configuration. Creating the Terraform resource adopts matching settings without commits, reloads, or reboot. Terraform identifiers and nonsecret settings are stored in state. Protect the local state directory and exclude it from version control.
3. Keep an external encrypted router backup and a verified management connection before applying. Do not overlap section ownership with other Terraform resources, the administration UI, or other tools.

Only the supplied options and lists are managed. Omitted settings remain unchanged. Remove a named section explicitly with `absent = true`; removing it from the configuration relinquishes ownership and preserves it. Destroy forgets the entire resource and leaves router settings intact. Protect production configurations with `prevent_destroy`.

Managed firewall rules are evaluated in configuration order. Refresh detects changes to their relative order. Applying moves the managed rules to the end of the package in the declared order, preserving unmanaged settings. Review unmanaged firewall rules before adopting a configuration: this resource does not infer a network security policy or remove rules outside its configuration.

## Rotate Wi-Fi keys

Use the sensitive write-only map `wireless_keys_wo`, keyed by managed named `wifi-iface` sections. Obtain values from ephemeral input variables and increase the positive `wireless_keys_version` when rotating. Keys are evaluated on creation and version changes; identical keys do not cause a router reload. They never enter saved plan or state artifacts. Omitting the map preserves existing keys. Refresh cannot detect key drift.

Nonsecret configuration rejects `key`, password, secret, token, and private-key option names. Do not place other credentials in arbitrary UCI options. See HashiCorp's [write-only argument contract](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments).

## Apply and recover

Changes take a private snapshot of all six supported packages on the router and arm a 300-second watchdog before staging or committing. Uncommitted UCI changes, a pending transaction, conflicting AdGuard DNS ownership, or unsupported hardware stop the operation. Configuration is staged and checked with `fw3` before the package commits. The watchdog restores all six files if the provider does not confirm the change.

Network and wireless changes reboot the router. A temporary init service restarts recovery after a reboot, including an unexpected reboot. Confirmed changes cancel recovery and remove the snapshot and service. The provider checks saved settings, managed rule order, SSH reconnection, declared active interfaces, selected wireless bridges, and the DNS process. Test client connectivity and the actual security policy independently after applying.

The transaction directory is `/root/.terraform-provider-glinet-configuration` on the router. Recovery does not restore credentials or data outside the six configuration packages. Power loss during file restoration, filesystem failure, or concurrent out-of-band writes require operator inspection and the external backup. Never bypass a host-key mismatch, remove a pending transaction, or reapply blindly after a failed confirmation.
