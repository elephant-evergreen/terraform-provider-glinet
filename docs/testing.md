# Verification boundaries

| Check | What it establishes |
| --- | --- |
| Terraform protocol tests with local router fixtures | Resource schema, create/update/read/import, refresh, drift handling, and write-only state behavior |
| Protocol tests with legacy state | A v0.3.0/v0.3.1 state plans and applies its first update without "Provider produced inconsistent result after apply"; a state with no other change plans no update |
| SSH server fixtures | Host-key rejection, bounded output, cancellation, and redacted transport failures |
| Backup fixtures | Standard age encryption and decryption, complete required archive contents, tamper/truncation rejection, private permissions, and failure before mutation |
| Recovery worker fixtures | Durable decision arbitration, cancellation, lost confirmation responses, restoration, reboot resumption, and cleanup behavior in a local command world |
| Recovery outcome model | Rapid drives the real restore script through failed package copies, failed service activation, retries, and repeated completion. It checks that `restored` is recorded only after every package and service is restored, that failures leave `restoring` with the snapshot, and that a retry converges to `restored` |
| Transaction outcome model | Rapid generates result, transaction-directory, lock, retired-directory, and recovery-service states and compares the real outcome probe with a model: a confirmed transaction reports `confirm` only after cleanup, a decided one stays `pending`, and a foreign transaction reports `superseded` |
| Concurrent clients | Two clients with the same stale ownership observation write the same section at once. The guard admits one and refuses the other with exit 78, and the recorded owner is the winner's token |
| Recovery failure fixtures | A failed copy or a failed service restart never reports a completed rollback; the router records `restore_failed` when it stops retrying and keeps the snapshot and recovery service. A lost confirmation response resolves to success only when the router recorded a confirmed transaction |
| Ownership fixtures | Forget clears the owner token on the router; a concurrent owner claim is refused with exit 78 before the transaction starts; the typed fixture applies the same token rules |
| Activation audit | Every package in the allowed UCI surface has a reload path, a service activation, a cloud call, or a recorded exemption; the SSH endpoint decision is tested separately |
| Property tests | Rapid generates attribute values, including non-default values filtered through the schema validators and retried until the resource's own validation accepts them. Each resource logs how many of the generated configurations were actually verified, and a resource that verifies none fails. Selector/load round trips preserve every explicit value, writes use only registered options, and the second round trip is stable. Write-only secrets never enter the state or another attribute. The managed-configuration merge keeps untouched sections identical, and the ordered firewall policy preserves the plan and converges when reapplied |
| Firewall fixtures and mutation controls | All filter and nat entry chains, declared/missing/reordered rules, nested counter-only branches, and rejection of undeclared actions or branch cycles |
| Native UCI fixture on a dedicated router | Actual libuci behavior in isolated temporary configuration; this does not demonstrate network activation or physical recovery |
| GoodCloud service fixtures | Stock `cloud.set_config` invocation, stop verification, service snapshot and restore, configuration/runtime mismatch detection, rollback failure retention, and rollback outcomes in a local command world |
| Transaction log fixtures | Each committed or rolled-back transaction records its phase durations, changed section count, commit count, reboot and restart counts, and whether it could cut the network |
| Documentation consistency | Every registered resource, data source, and action appears in the coverage matrix, and every type the matrix and the typed-resource guide reference is still registered |

Ordinary `terraform plan` reads the router and checks its generated and active firewall. Each write checks ownership and conflicts, creates a verified encrypted backup, protects the wired management route when needed, and confirms the managed interfaces, bridges, DNS process and firewall before committing the transaction outcome. These are configuration and activation checks, not a traffic acceptance suite.

## Independent hardware acceptance

Run `mise run acceptance` against an explicitly authorized Flint 2 (dedicated or the operator's) with stock firmware 4.11.x, an external encrypted recovery backup, and a tested wired recovery connection. The suite is opt-in: it requires `GLINET_ACCEPTANCE=1` and the `GLINET_*` connection variables, and it never runs in ordinary CI. Record the exact build in the private verification notes; the 4.11 line is the verified line.

The suite:

- verifies the model, the firmware line, router mode, and a clean transaction state;
- creates, updates, imports, and destroys a firewall redirect, a disabled OpenVPN server profile, a Samba share, and a firewall zone, and checks that each apply leaves an empty plan;
- injects an undeclared firewall rule during a redirect update, expects the transaction to roll back and the section to be restored, and verifies the injected rule is gone;
- changes the NTP configuration and checks that the stock time service restarts with a new process;
- runs a transaction whose confirmation response is discarded after the router executes it, and checks that the committed change is reported as success;
- reboots the router when `GLINET_ACCEPTANCE_REBOOT=1` is set and waits for the connection to return.

Each step removes the temporary objects it created and does not change singleton stock sections. A test failure before cleanup can leave an `Acceptance-*` object behind; remove it through the stock UI or the provider before the next run. Archive the sanitized result against the exact provider version. The suite is compiled by the ordinary check, so it cannot silently rot.

Traffic acceptance uses separate clients on each declared network. Check permitted and rejected network pairs, router management restrictions, private WAN ranges, DNS forwarding, and the declared management exceptions. It may switch SSIDs or use dedicated endpoints, but is never a dependency of normal Terraform operations. Synthetic probes and packet counters alone do not establish end-to-end connectivity.

Fixture success and read-only firewall captures do not establish physical power-loss recovery, a complete traffic matrix, or compatibility with other devices and firmware.

## Not yet verified on hardware

The fixture world runs the generated shell scripts, including the recovery worker and restore script, in a local command environment. It cannot prove the behavior of the physical device:

- a power loss during a configuration transaction, including the restore resume path;
- an SSH disconnect during a rollback, including the endpoint-changing SSH activation;
- a restore failure on the router (for example a full flash or a failed package copy);
- the actual restart count, stop time, and flash writes of a multi-resource apply.

The hardware acceptance suite exercises a rollback and the stock NTP restart. It also runs one automated lost-confirmation case: the confirmation command executes on the router and its SSH response is discarded, and the provider must report the committed change as success. Extend it with the remaining scenarios and record the results in the private verification notes before claiming them. A fixture result is not hardware verification.

## Manual recovery checks

Run these only on an explicitly authorized router with console or physical recovery access, and record the result against the exact build in the private verification notes. They are not part of `mise run acceptance` because they interrupt power or the management connection. Transaction state lives under `/root/.terraform-provider-glinet-configuration`; terminal results live under `/root/.terraform-provider-glinet-results/<token>`.

| Check | Procedure | Invariant to confirm |
| --- | --- | --- |
| Interrupted apply | Start an apply of a dedicated test resource and wait for `/root/.terraform-provider-glinet-configuration/armed` (or for `activate` after that), then remove power. Restore power and wait for SSH to return. Repeat at least five times across the phases (before activation, during the reload, after activation). | The test section holds either its previous or its new value, never a mix. The result record is one of `confirm`, `cancel`, `restored`, or `restore_failed`, or absent when the transaction never armed. The transaction directory and recovery service are gone after the recovery window, and the next plan is empty or shows only the intended change. |
| Lost confirmation | Run the acceptance suite's `lost_confirmation` case. | The apply succeeds, the section shows the new value, and the next plan is empty. |
| Disconnected provider | Start an apply that will roll back (the injected-rule pattern from `acceptanceRollback`), wait for `/root/.terraform-provider-glinet-configuration/armed`, and kill the provider process. Reconnect with a fresh client. | The router records `restored` (or `restore_failed` if the restore truly failed), the previous section value is back, and no `stuck` record remains once recovery finishes. Repeating the read is safe. |
| Restore failure | Use a disposable lab device and a controlled fault that makes a snapshot copy fail. Never run this on a router in use. | The result record is `restore_failed`, the snapshot and recovery service remain, and the provider reports that the router could not restore the configuration. |
