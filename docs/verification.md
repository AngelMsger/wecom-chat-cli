# Verification record

Version: 0.2.0-dev. Date: 2026-11-09. Toolchain: Go 1.27.3. All credentials and conversations in tests are synthetic.

## Completed checks

| Check | Result |
| --- | --- |
| Formatting and `go vet` (`make lint`) | Passed |
| Full unit/security tests with race detector (`make test`) | Passed on Linux amd64 |
| Compiled CLI against real agent/operator Unix sockets (`make e2e`) | Passed |
| Separate OS identity fixture (`make isolation-check`) | Passed on Linux amd64 |
| Generated command reference repeatability (`make docs-check`) | Passed |
| Native binary build (`make build`) | Passed |
| Linux/macOS amd64/arm64 cross-builds | All four passed; macOS native execution awaits CI |
| Dependency vulnerability scan (`make vuln`) | No vulnerabilities found after updating x/sys to v0.44.0 |
| npm checksum/platform/destination tests | All 3 passed |
| Release and repository setup contract tests | All 15 passed using local substitutes |
| Site links and generated narrative HTML | 9 pages passed; no broken local anchors/assets |
| Chromium site checks | 390/768/1440 px; task scenarios, clipboard, search and overflow checks passed |

Protocol tests exercise subscription credentials, exact callback request binding,
final stream payload, acknowledgement uncertainty, stale connection generations,
heartbeats, terminal authentication failure, displacement and failed intake.
Security/state tests cover malformed identities and JSON, unknown socket fields,
operator denial, trust envelopes, Chinese/English attack examples, dry runs,
single-event grants, cross-task denial, self-approval denial, immutable drafts,
exact-digest approval, one send across concurrent approvals, template-only auto
sends, risk reports without user blocking, grant rotation/revocation, session
serialization/reset, policy/connection/restart fencing, legacy migration,
expiry, capacity, cursors, process locks, scope mismatch, crash recovery, symlinks,
writable ancestors and persistence failure.

The CLI end-to-end fixture builds a real binary and checks JSON/NDJSON output,
operator watch cursor resume, environment-only worker grants, read-only intent,
dry-run, drafts that do not send, exact-digest review, one send across repeated
approvals, cross-task denial, session reset and risk reports without user bans.

The OS fixture starts a real broker as a trusted setup identity and executes a
probe as UID 2001/group 2000. Its task grant reads only Alice's event, while
Bob's content, inbox enumeration and operator operations remain unavailable.
The worker reaches its agent socket while operator
socket access, private state/secret reads and policy writes fail. Its effective
Linux capabilities are zero and outbound network is unavailable. This proves the
fixture's boundary, not the isolation of an unrelated grok bot deployment.

## Managed-environment test runner

This task's outer filesystem exposes `/` and `/tmp` as owned by UID 65534. The
broker intentionally rejects that environment instead of weakening its trusted
ancestor checks. Filesystem tests therefore ran in a conventional Ubuntu root
filesystem with networking disabled. The compiler ran outside the container;
test executables ran inside it. A pinned image and a reusable runner are included:

```sh
# Prefetch public dependencies and the test image using the host's configured
# network/proxy/CA settings before starting the network-disabled tests.
go mod download
docker --host=unix:///var/run/docker.sock pull \
  ubuntu@sha256:fe81560990fae86dc4743202b2cb4d3df98d0bcf949173cecef15ce1127ef852

GOFLAGS="-exec=$PWD/scripts/container-exec.sh" make test e2e
WECOM_CHAT_TEST_ROOT=1 GOFLAGS="-exec=$PWD/scripts/container-exec.sh" make isolation-check
```

The ordinary test container runs as the current UID, with a read-only root,
read-only source/module/toolchain mounts, no Linux capabilities and no network.
An executable temporary filesystem supports the compiled CLI fixture. The OS
isolation fixture mounts only its synthetic test executable; its root setup
process receives CHOWN/SETUID/SETGID solely to construct distinct identities, and
the worker has none. No Docker socket, host home or credential files are mounted.

Linux/macOS CI runs `make lint test e2e build docs-check site-check release-tests`
and installer/vulnerability checks directly. The optional
isolation test must use a disposable root setup environment and fails if invoked
as an ordinary user; it never silently reports an unexecuted check as passed.

## Not verified or delivered

- A live WeCom tenant, actual Bot ID/Secret, enterprise rollout and server reply
  validity window; no live messages were sent and no bot configuration changed.
- A concrete grok bot SDK/plugin adapter or the host's actual sandbox and tools.
- Disconnected-message replay/backfill or exactly-once model execution.
- General prompt-injection detection or factual-answer correctness; adversarial
  tests prove restricted effects, not semantic classification quality.
- The trustworthiness of an external automated reviewer or its credentials.
- Long-term ledger archival/compaction and high-volume durability/performance.
- Production systemd installation, private WeCom deployments, Windows sockets,
  macOS native execution, registry publication or a versioned release. The
  cross-build/checksum/npm pipeline is prepared and tested locally.

The source remains a prerelease. Remote CI, Pages deployment and repository
settings must be verified after the first push; local checks do not attest them. The hard event cap and explicit coverage
gaps must remain visible until a production retention/operations design is added.
