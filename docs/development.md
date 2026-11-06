# Development

Use the Go version in `go.mod` and Terraform 1.17.1. These tools are needed to develop the provider; installed provider packages do not require them at runtime.

```sh
bin/check-for-agent
go tool tfplugindocs generate --provider-name glinet
```

The check runs formatting, module verification, vet, race-enabled tests, compilation, and Terraform example formatting. Terraform protocol tests use the shipping resource implementation with local router fixtures. Ordinary checks never require router credentials or change a live network.

The CI workflow uses the same checks and verifies that generated documentation is committed. The release workflow builds the supported Linux and macOS architectures, produces Terraform-compatible archives and a protocol manifest, signs their checksums, and uploads versioned GitHub release assets. See [releases](releases.md).

Native UCI tests are gated by explicit test-device credentials. Physical recovery and traffic acceptance tests require a dedicated authorized router and independent wired recovery access. See [verification boundaries](testing.md).
