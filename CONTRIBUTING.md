# Contributing

Use English for documentation, examples, diagnostics, issues, pull requests, and commit messages.

Keep changes focused on the documented device and firmware scope. Model resources around OpenWrt objects and terminology. Propose additional device support with evidence for that device and firmware version.

## Licenses and attribution

Contributions are provided under the project's MPL-2.0 license. Preserve copyright and license notices when adapting third-party material, and record its source and version in NOTICE. Do not copy code or documentation whose redistribution terms are unclear.

## Test data

Use synthetic values in fixtures and examples. Do not commit router backups, configuration exports, credentials, private keys, Terraform state, plan files, or personal network identifiers.

Tests against a router must use a dedicated test device and explicitly documented credentials. Ordinary CI must not connect to a router or change a live network. A skipped hardware test is not evidence of compatibility.

## Changes

Explain the problem, the resulting behavior, and the verification performed in each pull request. Describe any unverified hardware behavior explicitly. Update documentation alongside changes to the public schema.

Run `mise install`, then `bin/check-for-agent`. The required checks use the tools pinned in mise.toml, run the provider protocol and local router-fixture tests, and do not require real router credentials. Run `mise run docs` after schema or example changes and commit the generated documentation.

The initial implementation supports Linux and macOS development environments. Additional operating systems require separate verification.
