# Migrating from glinet_uci_configuration

The coordinated JSON resource is deprecated. Migration is designed to preserve the router configuration and state.

## 1. Back up and freeze changes

Create an external encrypted backup and do not edit the router or run another Terraform apply during migration. Keep the old resource in configuration until the ownership marker is written.

## 2. Write ownership markers through the old resource

Add `adoption_version = 2` (or any higher value) to the existing resource:

```hcl
resource "glinet_uci_configuration" "home" {
  configuration         = jsonencode(local.sections)
  wireless_keys_wo      = var.wireless_keys
  wireless_keys_version = var.wireless_keys_version
  adoption_version      = 2
}
```

Run `terraform plan` and confirm the only expected change is the ownership marker. Apply. The provider writes `home_iac_owner = "1"` to every managed section in one recovery-protected transaction; marker-only changes do not reboot or reload network, wireless, DHCP, DNS, or firewall services. Run `terraform plan` again and confirm it is clean.

`adoption_version` is a one-time migration trigger. It is not needed by the typed resources.

## 3. Replace the old resource with typed resources

Remove `glinet_uci_configuration.home` from configuration and state:

```sh
terraform state rm glinet_uci_configuration.home
```

Add typed resources with import blocks. Example IDs:

```hcl
import {
  to = glinet_network_interface.lan
  id = "lan"
}

import {
  to = glinet_wireless_interface.private_2g
  id = "wifi2g"
}

import {
  to = glinet_firewall_rule.admin_nas
  id = "Home-IaC-Admin-NAS"
}
```

For anonymous stock sections, import by the stable option value, not the numeric UCI index:

```hcl
import {
  to = glinet_firewall_zone.lan
  id = "lan"
}
```

Run:

```sh
terraform plan -generate-config-out=generated.tf
```

Review `generated.tf`. Add references between resources (`network = glinet_network_interface.lan.name`) and keep `delete_behavior = "forget"` on network and management objects.

## 4. Apply and verify

Run `terraform plan` again, review the diff, then apply. The ownership marker is already present, so adoption should not reload services. A functional difference in the generated HCL will be applied with the normal recovery protection; use `prevent_destroy` on critical network objects.

## 5. Roll back

If the migration plan is not acceptable, restore the old configuration and the encrypted backup. The old resource and state can be restored with `git` and the backup; no router setting was deleted by state removal or typed import.
