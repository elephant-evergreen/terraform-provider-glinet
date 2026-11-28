# Repository instructions

Read [docs/README.md](docs/README.md) for the architecture, ownership, testing, compatibility, and release documents.

## Commands

- Install the pinned tools: `mise install`.
- Run the required checks: `mise run check`.
- Exercise the hardware acceptance suite against an explicitly authorized router: `GLINET_ACCEPTANCE=1 mise run acceptance`.
- Run the robustness session with `mise run fuzz` (`FUZZTIME` changes the duration) and the benchmarks with `mise run bench`.
- Regenerate the reference docs after schema or example changes: `go tool tfplugindocs generate --provider-name glinet`.

## Scope

Support Flint 2 (GL-MT6000) on stock firmware 4.11.x in router mode. Do not broaden hardware or firmware support without hardware evidence. See [compatibility](docs/compatibility.md).

## Code

- Use Go and the official Terraform Plugin Framework with the Go and Terraform versions in `mise.toml`.
- Do not add comments that restate the code. Record non-obvious invariants, ordering, and recovery reasons in comments or in [internal/router/AGENTS.md](internal/router/AGENTS.md).
- When changing an attribute's type or meaning, bump `SchemaVersion` and register a state upgrader. Do not rename or retype an attribute without one.
- Follow the [compatibility checks](CONTRIBUTING.md#changes) before designing or merging a change that affects existing installations.

## Tests and hardware

- Keep examples synthetic. Ordinary tests use local fixtures and never connect to a router.
- Hardware checks require an explicitly authorized device (dedicated or the operator's) and a verified recovery path. Ask the operator for explicit permission before every operation that can interrupt the network: network, Wi-Fi, VLAN, port, bridge, or route changes; router reboot; restarts of services that carry connectivity; and firewall, NAT, forwarding, or redirect changes that can cut traffic. Ask every time. A previous approval, a provider setting such as `allow_network_disruption`, or an environment variable is not consent.
- Record unverified hardware behavior in the issue or pull request. Fixture tests are not hardware compatibility.

## Public artifacts

- Write public documentation, examples, diagnostics, issues, pull requests, and commit messages in English.
- Follow the [public information rules](CONTRIBUTING.md#public-information). Never commit credentials, router exports, backups, Terraform state, personal network identifiers, or machine-specific paths.
- Preserve MPL-2.0 and third-party copyright notices.
- Include the reason for a change and a `Co-Authored-By: <tool> (<model>)` trailer in AI-assisted commits.
- Do not tag or publish a release without maintainer approval.

## CI

Keep one Check workflow. Add independent checks as parallel jobs inside it instead of creating another workflow. Keep `.github/workflows/actions.lock` in sync with the workflow `uses` refs: run `gh actions-lock` after changing a workflow, and let the Check workflow run `gh actions-lock --verify`.
