# Ownership and adoption

Every section the provider manages carries the `terraform_owner` option. The provider sets it to `1` while it manages the section and to `0` when `delete_behavior = "forget"` releases it.

## Current contract

| Operation | Behavior |
| --- | --- |
| Create | Writes the section and sets the marker. A stock singleton that already exists is adopted with its current settings, which is the normal way to bring the stock configuration under Terraform. |
| Import | Reads the section. The router keeps the marker it already has. |
| Update | Refuses a section whose marker is `0`. Otherwise writes the section and sets the marker to `1`. |
| Forget (destroy) | Sets the marker to `0` and keeps the section. |
| Delete | Removes a named section only while it carries marker `1`. Firmware-owned anonymous sections are never deleted. |

## Limitation

The marker is a boolean, so the provider cannot distinguish two Terraform states. Two states that manage the same section overwrite each other, and Create over an unmanaged named section adopts it silently because the stock configuration already exists.

## Planned owner tokens

1. Add a computed `owner` attribute to the typed resources. It holds a random token created on the first write; an import adopts a token from the router or generates one.
2. Store the token in the marker (`terraform_owner=<token>`). Legacy `1` states are adopted on their first write.
3. Read, Update, and Delete compare the state token with the router marker and stop with a conflict diagnostic when another state owns the section.
4. Roll out in a minor release with upgrade notes. The computed attribute does not change existing HCL, but the marker migration needs a documented window.

Until then, keep one Terraform state per router configuration. Use separate states only for disjoint sections and accept that the provider cannot detect a conflicting owner.
