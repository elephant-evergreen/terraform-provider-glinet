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

## SSH reuse evaluation

Reusing one SSH connection would reduce handshakes for management over a high-latency link. It is not implemented because a pooled connection changes failure semantics: a stale connection fails differently from a fresh one, and retrying a mutation after an ambiguous transport error is unsafe. The current model opens a fresh connection for every command, so a transport failure surfaces before the mutation starts.

The provider instead keeps the number of remote operations proportional to the resources being read, uses `singleflight` for concurrent reads, and avoids per-resource work that is independent of the change. Revisit connection reuse with an explicit keepalive and reconnect design, and measure it against a high-latency management path before adopting it.
