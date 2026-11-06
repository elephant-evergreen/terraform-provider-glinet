# Contributing

Contributions are welcome in any language. Maintainers write documentation, examples, diagnostics, issues, pull requests, and commit messages in English.

Keep changes focused on the documented device and firmware scope. Model resources around OpenWrt objects and terminology. Propose additional device support with evidence for that device and firmware version.

## Licenses and attribution

Contributions are provided under the project's MPL-2.0 license. Preserve copyright and license notices when adapting third-party material, and record its source and version in NOTICE. Do not copy code or documentation whose redistribution terms are unclear.

## Test data

Use synthetic values in fixtures and examples. Do not commit router backups, configuration exports, credentials, private keys, Terraform state, plan files, or personal network identifiers.

Tests against a router must use a dedicated test device and explicitly documented credentials. Ordinary CI must not connect to a router or change a live network. A skipped hardware test is not evidence of compatibility.

## Changes

Explain the problem, the resulting behavior, and the verification performed in each pull request. Describe any unverified hardware behavior explicitly. Update documentation alongside changes to the public schema.

Use the Go version declared in `go.mod` and Terraform 1.17.1. Run `bin/check-for-agent` for formatting, module verification, vet, race-enabled fixture and Terraform protocol tests, compilation, and Terraform example formatting. Ordinary checks do not require router credentials. Run `go tool tfplugindocs generate --provider-name glinet` after schema or example changes and commit the generated documentation. Release artifacts are built and signed by the release workflow.

The initial implementation supports Linux and macOS development environments. Additional operating systems require separate verification.
