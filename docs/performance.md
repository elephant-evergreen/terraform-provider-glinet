# Performance

The provider manages a single router over SSH. Each command opens its own connection, commands on one client are serialized, and concurrent reads share a single refresh through `singleflight`. An apply creates one transaction with an encrypted full backup, a request phase, a confirmation phase, and cleanup.

## Measured order

On a wired LAN with a configuration of about sixty resources, a read-only plan completes in roughly three seconds. Apply time is dominated by the encrypted backup, the configuration reloads, and the confirmation checks rather than by the provider's pure computation.

`mise run bench` reports the CPU-side costs. On a development machine the order was:

| Benchmark | Order |
| --- | --- |
| `quote` | tens of nanoseconds |
| `luaQuote` | about one microsecond |
| `mergeManagedConfiguration` | a few microseconds |
| `reconcileFirewallRules` (32 rules) | tens of microseconds |
| `configurationLua` (64 sections) | tens of microseconds |

Treat these as relative costs, not service levels.

## Measuring an apply

Set `GLINET_TRANSACTION_LOG` to a local path to append one JSON line per configuration transaction:

```sh
GLINET_TRANSACTION_LOG=/tmp/glinet-apply.jsonl terraform apply
```

| Field | Meaning |
| --- | --- |
| `result` | `committed`, `rollback:restored`, `rollback:restore_failed`, `unconfirmed`, and similar outcomes |
| `total_ms`, `backup_ms`, `apply_ms`, `reconnect_ms`, `verify_ms`, `confirm_ms` | Wall-clock durations on the provider host: the whole transaction, the encrypted backup, the apply command, the gap from the apply command to the first successful read (a management-path stop proxy, including a reboot), configuration and firewall verification, and the confirmation command |
| `changed_sections` | Number of changed UCI sections |
| `uci_commits` | Number of `uci commit` calls, one per touched package. Each commit may write the package file to flash, so this is a write-volume proxy. A transaction now commits only the packages its change touches instead of every managed package. |
| `reboots`, `restarts`, `restart_names` | Planned reboots, service activations, and which services were activated |
| `network_disruptive` | Whether the change required the network disruption permission |

The record contains no configuration values, secrets, or host identifiers. It is a local measurement aid and never changes the transaction outcome. The file is opened with mode `0600` and appended one line per transaction.

A plan with one resource stops the router for at most one reboot plus its service activations. A multi-resource apply serializes one transaction per resource, so the total stop time is the sum of the per-resource transactions and the restarts are not merged. Use the log to count both before changing the apply order, and keep the one-resource-per-transaction boundary when the alternative would remove the recovery window.

The transaction writes only the packages named by the change and commits only those; the rollback snapshot still covers every managed package. Compare `uci_commits` with the number of changed packages to confirm that a transaction did not rewrite unrelated configuration.

These numbers were not yet collected on hardware. Fixture runs establish that the fields are recorded and that service and reboot counts match the plan; they do not establish device stop time or flash wear.

## SSH reuse evaluation

Reusing one SSH connection would reduce handshakes for management over a high-latency link. It is not implemented because a pooled connection changes failure semantics: a stale connection fails differently from a fresh one, and retrying a mutation after an ambiguous transport error is unsafe. The current model opens a fresh connection for every command, so a transport failure surfaces before the mutation starts.

The provider instead keeps the number of remote operations proportional to the resources being read, uses `singleflight` for concurrent reads, and avoids per-resource work that is independent of the change. Revisit connection reuse with an explicit keepalive and reconnect design, and measure it against a high-latency management path before adopting it.
