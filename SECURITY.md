# Security

## Reporting

Report a vulnerability privately through GitHub's **Report a vulnerability** action on this repository. Do not open a public issue for a vulnerability that could expose router credentials, bypass host-key verification, or alter a router without authorization.

Include the provider version, the router model and firmware line, the affected resource, and a minimal reproduction with synthetic values. Do not include private keys, passwords, SSIDs, or router exports.

## Trust model

- The provider connects to the router as `root` over SSH and requires host-key verification against the configured known-hosts file. Host-key checking cannot be disabled.
- The SSH private key decrypts the age-encrypted router backups. Treat it as the recovery credential and keep it outside Terraform state and version control.
- Wi-Fi keys, PPPoE passwords, VPN keys, tokens, and similar options are write-only or sensitive in the schema, are passed to the router separately from the UCI configuration, and are excluded from the safety state the provider reads back.
- Terraform state can still contain non-secret topology and configuration values. Store state in encrypted remote storage.
- The provider refuses to replace firmware include scripts, rejects unsupported UCI scopes, and requires an encrypted backup before the first change in an apply.
- The provider does not manage GL.iNet account binding, provider profiles, or cloud activation.

## Supported platform

See [compatibility](docs/compatibility.md) for the supported model, firmware line, and operating systems.
