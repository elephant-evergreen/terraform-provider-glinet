# Migrating from glinet_uci_configuration

The coordinated JSON resource is deprecated. Migration is designed to preserve the router configuration and state.

## 1. Back up and freeze changes

Create an external encrypted backup and do not edit the router or run another Terraform apply during migration. Keep the old resource in configuration until all imports succeed.

## 2. Add typed resources with import blocks

Use import IDs from [typed resources](typed-resources.md#resources). For example:

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

For anonymous stock sections, use the stable option value, not the numeric UCI index:

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

Review `generated.tf`. It is a starting point; add references between resources (`network = glinet_network_interface.lan.name`) and keep `delete_behavior = "forget"` on network and management objects.

## 3. Remove the old state entry

After the typed resources are imported and the configuration is complete:

```sh
terraform state rm glinet_uci_configuration.home
```

Do not destroy the old resource through Terraform; its destroy behavior is forget, but removing state first is explicit and avoids any chance of a coordinated write.

## 4. Adopt ownership markers

Run `terraform plan`. The first plan should show only the provider ownership marker and any explicit attribute corrections. Apply once; marker-only changes do not reload network, wireless, DHCP, DNS, or firewall services. Verify that the plan is clean afterward.

## 5. Roll back

If the migration plan is not acceptable, restore the old configuration and the encrypted backup. The old resource and state can be restored with `git` and the backup; no router setting was deleted by the import or state removal.
