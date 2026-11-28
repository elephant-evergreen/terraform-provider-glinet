# Router package

Read [the design](../../docs/design.md) before changing the transaction, safety, or recovery code.

## Transaction invariants

- Never mutate configuration before the encrypted backup and the management-path protection succeed.
- The apply script writes the restore script, the watchdog, and the request before it touches configuration. The watchdog waits for `activate`, and the provider waits for `ready`.
- Verification compares the written configuration and the running firewall with the generated policy, then the confirmation runs the runtime checks again.
- The provider requests rollback on failure. The router-side watchdog restores the snapshot when no decision arrives within the recovery window. Never report success for an unconfirmed change.
- A decided rollback is not a completed one. The restore script records `restoring` before it starts, `restored` only after every snapshotted package and the same service activation as the apply have finished, and nothing else while it retries. The watchdog records `restore_failed` when it gives up. The provider reports a rollback only for `restored`.
- A rollback must not acknowledge a successful apply, and a confirmed transaction must not be rolled back afterwards. Terminal outcome records never change back.
- The transaction lock and the boot-id readiness check reject stale or concurrent participants.
- Prepare the transaction in the staging directory and publish it with one atomic rename only after the recovery service is enabled and synced. A power loss before the rename must leave no transaction directory; the boot service discards the staging area.
- The ownership claim records the owner token observed before the write, and it is checked again inside the transaction lock, before the transaction is prepared, with exit code 78. Cover every create, update, delete, and forget, including a section without a token and an anonymous section that does not exist yet (use the deterministic create name). A lost token is rewritten only while the managed marker is `1`; an explicit release (marker `0`) or a missing marker is refused. Any check that mutates configuration before it can refuse a concurrent owner is wrong.
- An SSH endpoint change (disable or a port other than the provider's management port) is refused. Never activate a change the provider cannot observe, and never report its result as verified.
- A confirmed transaction counts as committed only after its cleanup removed the transaction directory, lock, and recovery service. The probe and the confirmation acknowledgement use the same cleanup conditions.

## Safety invariants

- Ownership markers and owner tokens decide what the provider may change. Never overwrite or delete a section owned by another state.
- Refuse unmanaged zones, forwardings, redirects, ACCEPT rules, or interface changes that could reclassify managed networks. Do not weaken these checks to make a change apply.
- The safety state never carries passwords, keys, PSKs, tokens, or other secrets.

## Service activation

A changed package restarts its stock service and waits for the process when the service is enabled. Rollback runs the same activation against the restored configuration.

## Tests

- Use the `newConfigurationWorld` fixture for scripts and the typed fixture for resources. Do not connect to a router in unit tests.
- Add a regression test for any transaction, rollback, or safety change. Fuzz targets live in `fuzz_test.go`.
- Keep the recovery model in `recovery_model_test.go` and the activation audit in `services_test.go` passing when a package or restore step changes.
- `SetTransactionLog` is a diagnostic aid only. It must never change a transaction outcome or record configuration values.
