# Ownership and adoption

Every section the provider manages carries the `terraform_owner` option. The provider sets it to `1` while it manages the section and to `0` when `delete_behavior = "forget"` releases it.

## Current contract

| Operation | Behavior |
| --- | --- |
| Create | Writes the section and sets the marker. A stock singleton that already exists is adopted with its current settings, which is the normal way to bring the stock configuration under Terraform. |
| Import | Reads the section. The router keeps the marker it already has. |
| Update | Refuses a section whose marker is `0`. Otherwise writes the section and sets the marker to `1`. |
| Forget (destroy) | Sets the marker to `0`, clears the owner token, and keeps the section. |
| Delete | Removes a named section only while it carries marker `1`. Firmware-owned anonymous sections are never deleted. |

## Owner tokens

Each typed resource also records an owner token in the `terraform_owner_id` option and in the computed `owner` attribute. The token does not appear in configuration.

| Operation | Behavior |
| --- | --- |
| Create | Generates a token. A section that already carries another token is refused with "owned by another configuration". |
| Update | Compares the router token with the state token and stops when they differ. A section without a token (legacy marker `1`) is adopted and receives a token. |
| Import | Adopts the token already on the router. An unmanaged section has no token until the next write. |
| Delete | Compares the tokens before removing or releasing the section. |
| Forget | Clears the marker and the token on the router. Releasing a section and creating it again does not see a stale owner. |

A provider without owner-token support can rewrite a section without the token. The next write with token support regenerates it.

## Concurrent writers

Reading the owner and writing the section are not one step, so two Terraform states could both observe an unowned section and then write it in turn. The provider therefore records the owner token it observed and re-checks it after the router-side transaction lock is acquired and before it prepares the transaction:

- The claim holds the token currently on the router, or an empty token when the section has none or does not exist yet.
- The router token must still match. A token that disappeared is rewritten, because a provider without token support can drop it; a token that appeared is refused.
- An anonymous section that does not exist yet is claimed by the deterministic name the provider creates, so two states cannot both create it.

This covers Create, Update, Delete, and Forget, including a legacy state without a token. A section claimed by another state in between is refused with "owned by another Terraform state", before any configuration changes, so the plan can be re-run without inspecting a partially applied transaction.

## Legacy states

States written before owner tokens existed have `terraform_owner = 1` and no token. The next change to such a resource plans `owner` as unknown, writes a token in the same transaction, and records it in the state. A plan with no other change stays empty; the token is adopted on the first real change. After that first change, plans are empty again.

## Limitation

A boolean marker alone cannot distinguish two Terraform states. Owner tokens cover sections created by a state, but importing a section that another state already manages adopts its token, so both states treat each other as the same owner. Importing is an explicit adoption decision. Keep one Terraform state per router configuration and use separate states only for disjoint sections.
