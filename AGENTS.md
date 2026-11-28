# Repository instructions

- Write public documentation, examples, diagnostics, issues, pull requests, and commit messages in English.
- Preserve MPL-2.0 and third-party copyright notices. Do not add code comments unless explicitly requested.
- Never commit credentials, router exports, backups, Terraform state, personal network identifiers, or machine-specific paths.
- Follow the [public information rules](CONTRIBUTING.md#public-information) for documentation, examples, issues, pull requests, commit messages, and release notes.
- Support existing Wi-Fi AP settings, coordinated UCI configuration for network, wireless, DHCP, firewall, stock DNS, Ethernet port mapping, and the stock GoodCloud operation on Flint 2 stock firmware 4.11.x in router mode. Firmware 4.11.x in router mode is the hardware-verified line; other minors are rejected. Do not broaden hardware or firmware support without evidence.
- Use Go and the official Terraform Plugin Framework. Use the Go and Terraform versions in mise.toml, and run mise run check for required checks.
- Keep examples synthetic. Ordinary tests must use local test fixtures and must not change a real router. Hardware acceptance testing requires an explicitly authorized dedicated device.
- Record unverified hardware behavior in the issue or pull request. Do not equate fixture tests with hardware compatibility.
- When changing an attribute's type or meaning, bump `SchemaVersion` and register a state upgrader on the typed spec. Do not rename or retype an attribute without one.
- Follow the [compatibility checks](CONTRIBUTING.md#changes) before designing or merging changes that affect existing installations.
- Include the reason for a change and a Co-Authored-By trailer identifying the AI tool and model in AI-assisted commits.
