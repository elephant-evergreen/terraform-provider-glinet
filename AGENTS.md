# Repository instructions

- Write public documentation, examples, diagnostics, issues, pull requests, and commit messages in English.
- Preserve MPL-2.0 and third-party copyright notices. Do not add code comments unless explicitly requested.
- Never commit credentials, router exports, backups, Terraform state, personal network identifiers, or machine-specific paths.
- Keep the initial scope to existing named Wi-Fi AP sections on Flint 2 stock firmware 4.11.0 in router mode. Do not broaden hardware or firmware support without evidence.
- Use Go and the official Terraform Plugin Framework. Pin development tools in mise.toml and use bin/check-for-agent for required checks.
- Keep examples synthetic. Ordinary tests must use local test fixtures and must not change a real router. Hardware acceptance testing requires an explicitly authorized dedicated device.
- Record unverified hardware behavior in the issue or pull request. Do not equate fixture tests with hardware compatibility.
- Include the reason for a change and a Co-Authored-By trailer identifying the AI tool and model in AI-assisted commits.
