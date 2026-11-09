# Agent guide

This is a prerelease WeCom smart-bot bridge, separate from Calendar and Channel.
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
