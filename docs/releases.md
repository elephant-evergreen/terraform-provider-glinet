# Releases and Registry publication

Terraform installs `elephant-evergreen/glinet` from Terraform Registry using a fixed semantic version and `.terraform.lock.hcl`. Users do not build or install provider executables themselves.

The release workflow runs provider checks, builds Linux and macOS packages for amd64 and arm64 with CGO disabled, produces a versioned registry manifest, writes SHA-256 checksums, and creates a detached GPG signature. The manifest advertises protocol 6.0. The workflow uses GoReleaser 2.20.0 and actions pinned to commit IDs. Release credentials are available only to the tag-triggered release job; pull-request checks have read-only repository permissions.

## First publication

The configured release public key is [keys/release.asc](../keys/release.asc), fingerprint `9699ECB55B3E8B042C68A4B063527B92B224D3DD`. The encrypted private key and passphrase are configured as repository Actions secrets; neither is part of this repository. Namespace registration remains a separate authenticated action.

1. Store the release signing key in the repository Actions secret `GPG_PRIVATE_KEY` and its passphrase in `PASSPHRASE`. Back up this identity outside Git. Upload only its public key to the publisher's HCP Terraform public namespace.
2. Register the public `elephant-evergreen/terraform-provider-glinet` repository under the `elephant-evergreen` Registry namespace using an authorized HCP Terraform account. This step requires namespace ownership and Registry authentication.
3. Create a semantic version tag, such as `v0.1.0`, from a reviewed default-branch commit whose checks passed. The release workflow owns the build and publication of GitHub assets.
4. Confirm Registry ingestion, then run `terraform init` in a fresh directory using the pinned provider version. Verify the downloaded signature and checksums, and commit the generated dependency lock file in the consuming configuration.

A successful build, GitHub release, or local test installation does not establish Registry availability. Do not publish a usable-release compatibility claim without the [required hardware evidence](testing.md).

The artifact layout follows the [HashiCorp publication contract](https://developer.hashicorp.com/terraform/registry/providers/publishing) and [official scaffold](https://github.com/hashicorp/terraform-provider-scaffolding-framework).
