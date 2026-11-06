# Verification boundaries

| Check | What it establishes |
| --- | --- |
| Terraform protocol tests with local router fixtures | Resource schema, create/update/read/import, refresh, drift handling, and write-only state behavior |
| SSH server fixtures | Host-key rejection, bounded output, cancellation, and redacted transport failures |
| Backup fixtures | Standard age encryption and decryption, complete required archive contents, tamper/truncation rejection, private permissions, and failure before mutation |
| Recovery worker fixtures | Durable decision arbitration, cancellation, lost confirmation responses, restoration, reboot resumption, and cleanup behavior in a local command world |
| Firewall fixtures and mutation controls | All filter and nat entry chains, declared/missing/reordered rules, nested counter-only branches, and rejection of undeclared actions or branch cycles |
| Native UCI fixture on a dedicated router | Actual libuci behavior in isolated temporary configuration; this does not demonstrate network activation or physical recovery |

Ordinary `terraform plan` reads the router and checks its generated and active firewall. Each write checks ownership and conflicts, creates a verified encrypted backup, protects the wired management route when needed, and confirms the managed interfaces, bridges, DNS process and firewall before committing the transaction outcome. These are configuration and activation checks, not a traffic acceptance suite.

## Independent hardware acceptance

Use an explicitly authorized dedicated Flint 2 with stock firmware 4.11.0, an external encrypted recovery backup, and a tested wired recovery connection. Keep credentials and router exports outside Git and public logs.

Verify an actual CRUD/import/drift cycle, clean plans after apply, an intentional failed confirmation and rollback, and recovery across SSH disconnection and reboot. Check default policies and generated/running firewall differences with one declared redirect. Archive sanitized results against the exact provider version.

Traffic acceptance uses separate clients on each declared network. Check permitted and rejected network pairs, router management restrictions, private WAN ranges, DNS forwarding, and the declared management exceptions. It may switch SSIDs or use dedicated endpoints, but is never a dependency of normal Terraform operations. Synthetic probes and packet counters alone do not establish end-to-end connectivity.

Fixture success and read-only firewall captures do not establish physical power-loss recovery, a complete traffic matrix, or compatibility with other devices and firmware.
