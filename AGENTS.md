# Agent guide

This is a WeCom smart-bot bridge, separate from Calendar and Channel.
Read CONTRIBUTING.md and docs/technical-design.md before changes. Use English
for engineering material and agent-cli-kit for output, errors and argv handling.

The broker holds credentials; its worker interface is untrusted. Only trusted
supervisor/reviewer code may access the operator socket. Worker operations require
a short-lived single-event token: no inbox enumeration, target selector or self-approval. Authority comes
from separate OS-protected sockets, never request fields or chat identity. Never
add arbitrary URLs, shell hooks, file access, credential export or self-approval.
Every external record keeps its fixed trust envelope. Filters are heuristics,
not proof of safety. A sandboxed worker with no private data is mandatory.

Replies bind to the stored event, connection generation and a local deadline.
Persist sending before network I/O; ambiguous results are terminal and never
replayed. Risk reports bind to the event and never change user permissions. All quarantine,
block/unblock and session resets require the operator interface. Free-text drafts
require exact-digest independent approval. Only explicit literal-template policy
may auto-send; never add free-text fallback or template interpolation. Recheck
policy, epoch, expiry, connection and revocation before every task operation/send.
Keep same-session grants serialized and invalidate stale work on session reset.
Keep state locked, scope-bound, bounded and private; validate all path ancestors.

Keep idle monitoring outside model turns. A trusted ordinary host supervisor owns
the durable inbox, event dedupe, enqueue cursor and model-job lifecycle; the broker
does not invoke a model or expose arbitrary webhook/command hooks. A separate
`relay run` process may own collection and fixed-destination delivery; never put
its configuration, sender key, operator socket or cross-session claims in a model
worker. Port Channel delivery semantics, not its authorization or event DTOs.
Keep explicit ACK separate from HTTP acceptance; persist sending, unknown outcomes,
terminal errors and pacing across restart. Supervisor claims deduplicate batches
and events, retain recoverable metadata and never expire automatically. Discard
only never-attempted expired notifications; preserve a lifetime attempt marker
across retry-budget resets, distinct message IDs and
session epochs. Issuance must still succeed before starting a conversation worker.
Test actual CLI restart, unknown outcomes, duplicate claims and expired batching. Watch defaults
to a quiet baseline and pending metadata references; explicit replay and resume
must preserve feed/ledger/policy binding. Output progress is only `emitted`, never
accepted or processed. Do not reissue existing grants or collapse distinct messages
to reduce cost. Keep busy-session work queued until issuance succeeds. Test idle
and filtered periods, duplicate callbacks, cursor scope, bounded-process resume,
closed/expired tasks and restart recovery. Follow docs/host-integration.md and the
family's event-driven-agent-integration guide before changing this contract.
Keep pending discovery linear per snapshot: index the latest assignment by session
and epoch, including revoked/closed tasks, instead of rescanning suffixes for each
candidate. Fit server long polls inside the caller's remaining observation budget
with time to return filtered progress; never advance a cursor past un-emitted work.
Cover large unrelated histories with a benchmark and filtered arrivals during a
short resumed CLI process with a regression test.

Run make lint test e2e build docs-check site-check release-tests, npm test --prefix
build/npm, and make vuln. Tests use synthetic credentials and local
WebSocket peers only. Do not activate a live bot, push or publish for validation.
Document unsupported capabilities and unverified live behavior explicitly.

For OS-isolation changes, also run the tagged isolation fixture inside a
disposable network-disabled Linux container; see docs/verification.md. Do not
relax trusted ancestor checks to accommodate an executor with mapped root owners.

Repository infrastructure follows Calendar/Channel: Markdown is canonical; make
docs generates HTML and the existing per-command Markdown URLs. Keep site assets
local and prerelease claims accurate. Tag-driven releases use the family GitHub
Release/checksum/npm OIDC pipeline, with no additional approval environment. Never
publish a tag just for validation. The npm publisher requires separate package-owner
setup before the first release; do not claim it exists without checking. Use the
kit for shared behavior, pkg/constants for release metadata, and the existing
checksum-required installer. Keep Unix-only targets and the no-runtime-download rule.
After the initial push, use a branch and PR; never bypass main protection.
The repository setup script is limited to AngelMsger/wecom-chat-cli and never
changes sibling repositories. Read docs/releasing.md before changing that contract.
For macOS tests canonicalize temporary fixture paths and use short Unix socket
paths; never weaken production symlink/ancestor checks to make CI pass.

Keep the public README aligned with the family: poster and slogan, feature and
scope explanation, installation/quick start, commands, configuration, output and
error contracts, Skill, development, documentation, related tools and license.
Keep first-release status factual. README badges belong only inside the marked
GitHub-only block; generated HTML must keep using local assets. Preserve accurate
role/deadline/security limits when reorganizing introductory material. Record a
poster generation prompt and update the canonical docs/image.png, never a sibling image.

Do not latch HTTP outcome uncertainty as a terminal delivery error: reconciliation
of verified non-acceptance resumes the remaining budget without a generic reset.
