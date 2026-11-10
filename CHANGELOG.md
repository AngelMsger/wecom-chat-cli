# Changelog

## Unreleased

## [0.3.0] - 2026-11-10

- Index the latest assignment per session/epoch once per watch snapshot, retaining
  supersession by revoked/closed tasks without repeated suffix scans or session
  hashes. Bound watch long polls by the remaining invocation time, preserving
  filtered progress before the CLI deadline instead of rescanning it next run.

- Make `event watch` quiet by default, with explicit `--replay` and a separate
  policy/ledger-bound pending feed. Emit metadata references for live unassigned
  messages, retaining full content in scoped task and operator audit reads.
  Exclude closed, assigned, expired and stale work before host scheduling; preserve
  later messages in busy sessions for ordinary host queuing.
- Keep filtered long-poll wakeups silent and return resumable idle checkpoints.
  Rename the misleading watch `delivered` count to `emitted`, explicitly marking
  the checkpoint stage. Legacy watch/list cursors require deliberate migration.
- Document the actual pull-based supervisor sequence, zero-idle-inference target,
  transactional host enqueue/dedupe and ambiguous-job recovery. No Grokbot SDK,
  webhook trigger or end-to-end exactly-once guarantee is implied.

## [0.2.0] - 2026-11-10

- Adopt agent-cli-kit v0.9.0 after reviewing Dependabot PR #1 and validating the
  pinned dependency. Its changed HTTP redirect/doctor path is not used by Chat's
  dedicated WebSocket and Unix-socket transports.
- Align the public README with the CLI family: product poster, badges, features,
  official-CLI positioning, installation, quick start, commands, output/error
  contracts, Skill, development, documentation, related tools and license.
- Publish the GitHub Pages documentation site and add the product poster to the
  generated operator guide and site share metadata. No binary/npm release is implied.

- Pin the family Go 1.27.3 toolchain and update x/sys to v0.44.0, removing a
  Windows-only module advisory even though Windows is not a supported target.
- Add public-repository bootstrap, Linux/macOS CI, isolated Linux checks, Pages,
  issue/PR templates, private security reporting and kit dependency updates.
- Generate a searchable static documentation site while preserving existing
  per-command Markdown URLs; add host/task explorer and family navigation.
- Prepare tag-driven native releases, SHA-256 assets and the npm OIDC installer;
  inject version/commit/build time through the family build pipeline. No release
  or registry publication is performed by repository bootstrap.

- Require capability-authenticated, single-event worker operations. Full inbox,
  grant issuance/revocation and session reset now require the operator socket.
- Bind grants to event, policy, session epoch, broker/connection generation and
  expiry; serialize same-session work and fence stale results after reset.
- Replace direct sends with immutable drafts and exact-digest independent review.
  Default policy is review; optional automatic replies use reviewed literals only.
- Replace model-controlled quarantine with immutable risk reports that cannot
  change user permissions. Operator quarantine/block/watch/clear remains available.
- Introduce strict version 2 policy and migrate version 1 ledgers while preserving
  replay history, memory and send outcomes. Legacy events cannot acquire grants.
- Retain token hashes only; disclose a grant once to the operator and read worker
  credentials from the dedicated environment, never argv or JSON bodies.
- Extend adversarial authorization, send-outcome, migration, CLI/socket and distinct
  OS-identity tests. Update host integration, security guidance and command reference.

## Initial local prototype

Added WebSocket intake, provenance, bounded watches, protected durable state,
reply deduplication, heuristics and OS-separated sockets. The prototype's direct
send and model quarantine commands are superseded by the version 2 contract.
