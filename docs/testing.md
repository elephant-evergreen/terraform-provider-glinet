# Verification boundaries

| Check | What it establishes |
| --- | --- |
| Terraform protocol tests with local router fixtures | Resource schema, create/update/read/import, refresh, drift handling, and write-only state behavior |
| SSH server fixtures | Host-key rejection, bounded output, cancellation, and redacted transport failures |
| Backup fixtures | Standard age encryption and decryption, complete required archive contents, tamper/truncation rejection, private permissions, and failure before mutation |
| Recovery worker fixtures | Durable decision arbitration, cancellation, lost confirmation responses, restoration, reboot resumption, and cleanup behavior in a local command world |
| Firewall fixtures and mutation controls | All filter and nat entry chains, declared/missing/reordered rules, nested counter-only branches, and rejection of undeclared actions or branch cycles |
| Native UCI fixture on a dedicated router | Actual libuci behavior in isolated temporary configuration; this does not demonstrate network activation or physical recovery |
| GoodCloud service fixtures | Stock `cloud.set_config` invocation, stop verification, service snapshot and restore, configuration/runtime mismatch detection, rollback failure retention, and rollback outcomes in a local command world |

Ordinary `terraform plan` reads the router and checks its generated and active firewall. Each write checks ownership and conflicts, creates a verified encrypted backup, protects the wired management route when needed, and confirms the managed interfaces, bridges, DNS process and firewall before committing the transaction outcome. These are configuration and activation checks, not a traffic acceptance suite.

## Independent hardware acceptance

Run `mise run acceptance` against an explicitly authorized dedicated Flint 2 with stock firmware 4.11.x, an external encrypted recovery backup, and a tested wired recovery connection. The suite is opt-in: it requires `GLINET_ACCEPTANCE=1` and the `GLINET_*` connection variables, and it never runs in ordinary CI. Record the exact build in the private verification notes; the 4.11 line is the verified line.

The suite:

- verifies the model, the firmware line, router mode, and a clean transaction state;
- creates, updates, imports, and destroys a firewall redirect, a disabled OpenVPN server profile, a Samba share, and a firewall zone, and checks that each apply leaves an empty plan;
- injects an undeclared firewall rule during a redirect update, expects the transaction to roll back and the section to be restored, and verifies the injected rule is gone;
- reboots the router when `GLINET_ACCEPTANCE_REBOOT=1` is set and waits for the connection to return.

Each step removes the temporary objects it created and does not change singleton stock sections. A test failure before cleanup can leave an `Acceptance-*` object behind; remove it through the stock UI or the provider before the next run. Archive the sanitized result against the exact provider version. The suite is compiled by the ordinary check, so it cannot silently rot.

Traffic acceptance uses separate clients
Traffic acceptance uses separate clients on each declared network. Check permitted and rejected network pairs, router management restrictions, private WAN ranges, DNS forwarding, and the declared management exceptions. It may switch SSIDs or use dedicated endpoints, but is never a dependency of normal Terraform operations. Synthetic probes and packet counters alone do not establish end-to-end connectivity.

Fixture success and read-only firewall captures do not establish physical power-loss recovery, a complete traffic matrix, or compatibility with other devices and firmware.
