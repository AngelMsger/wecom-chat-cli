# WeCom Chat CLI

Built for agents. Designed for humans.

A credential-holding broker between a WeCom smart bot and isolated conversation
workers, including hosts such as grok bot. It maintains the official WebSocket,
labels external input as untrusted, issues single-event task capabilities, and
requires independent review before sending a model's free-text answer.

**Status: 0.2.0-dev prerelease; no binary/npm release or live-tenant certification.**
[Website](https://angelmsger.github.io/wecom-chat-cli/) · [GitHub](https://github.com/AngelMsger/wecom-chat-cli) · [Release guide](docs/releasing.md)

See [assessment](docs/assessment.md), [design](docs/technical-design.md),
[security](docs/security.md), [host integration](docs/host-integration.md) and
[verification](docs/verification.md). Calendar and Channel remain separate CLIs.

## Protection that does not depend on model judgment

- Workers cannot enumerate the inbox, choose an event/recipient, issue grants,
  approve their own drafts, reset sessions or change user permissions.
- Each short-lived grant binds to one event, sender/conversation, session epoch,
  operator policy and current broker/WebSocket generation. Only a hash is stored.
- Same-session tasks are serialized. Resetting a session or revoking a task stops
  old work; policy changes, blocked users, expiry and reconnects also invalidate it.
- A free-text draft is immutable. The operator must inspect its target and full
  text, then approve that exact digest before the broker sends it.
- Model risk reports close that event's unsent reply path but never quarantine or
  block a person. Only the operator changes risk memory.
- Sends persist `sending` before I/O; duplicates return the known outcome and
  ambiguous sends are never replayed automatically.

Text heuristics remain a preliminary filter. They do not certify safety or answer
correctness. Even a completely compromised worker must lack administrator tools,
private data and other workers' grants. See the required [sandbox](docs/security.md).

## Reply policies

| `reply_mode` | Worker input | Sending authority |
| --- | --- | --- |
| `review` (default) | A final text draft | Independent operator approval of its immutable digest |
| `template` (explicit opt-in) | An approved template ID | Broker sends the exact operator-configured literal; no variables or substitutions |
| `disabled` | No reply proposal | No sends; task reading and risk reporting remain available |

There is no unrestricted automatic free-text mode. Template selection can still
be inappropriate; configure only harmless public responses suitable for every
allowed recipient, such as a service-desk handoff. A stronger reviewer model can
assist an operator, but is neither built in nor a substitute for access controls.

## Build and verify

Go 1.25+ with the pinned toolchain from `go.mod`. CI targets Linux and macOS;
Linux is the production isolation reference. No published npm version exists yet.

```sh
git clone https://github.com/AngelMsger/wecom-chat-cli.git
cd wecom-chat-cli
```

```sh
make build
make lint test e2e docs-check site-check release-tests
npm test --prefix build/npm
bin/wecom-chat-cli --help
```

Tests use synthetic conversations and local peers, not WeCom. The optional
network-disabled container runner and OS-identity fixture are documented in
[verification](docs/verification.md). No live bot credentials were used.

## Operator provisioning

Create a dedicated smart bot with API long-connection mode in WeCom administration.
Obtain Bot ID and Secret there. Inspect existing integrations first: a competing
subscriber can displace a connection, and switching API mode can disable a webhook.

Provision a broker identity, isolated workers, protected policy/state, and separate
worker/operator socket parents using [security](docs/security.md) and the
[systemd reference](deploy/wecom-chat.service). Give only the trusted non-model
supervisor/reviewer access to the operator socket. Workers receive the agent socket
plus their own grant, never the operator socket or the bot Secret.

```sh
bin/wecom-chat-cli config example > broker.json
# Review version 2, bot/deployment, paths, stable user/group IDs and reply_mode.
bin/wecom-chat-cli config validate --policy /absolute/protected/broker.json
bin/wecom-chat-cli broker serve --policy /absolute/protected/broker.json --dry-run
```

The template has empty allowlists and `reply_mode: review`. Stable user IDs come
from trusted directory administration or the official CLI's directory discovery
(`wecom-cli contact --help`), never display names. Group IDs are an explicit
prerequisite supplied by the trusted deployment owner. Unknown identities are not
admitted merely to discover them.

Supply `WECOM_CHAT_BOT_SECRET` only in the broker's protected service environment,
then deliberately start `broker serve --policy /etc/wecom-chat/broker.json`.
The CLI does not provision accounts, fix privileged permissions or install services.

## Operator: receive an event and issue a task

```sh
wecom-chat-cli --socket /run/wecom-chat-operator/bridge.sock \
  --bot-id BOT_ID --deployment support event watch --duration 60s --max-events 100

wecom-chat-cli --socket /run/wecom-chat-operator/bridge.sock \
  --bot-id BOT_ID --deployment support task issue --event EVENT_ID --allow-writes
```

The issue response contains a **secret `task_token`, disclosed once**. The trusted
supervisor captures it without logging it or exposing it to the model, and injects
`WECOM_CHAT_TASK_TOKEN` into only that worker's tool executor. Never put a token in
argv, a shared environment, chat history or a general credential store. If delivery
of the grant is uncertain, inspect `event get`, revoke it and issue a replacement;
issuance is bounded to eight grants per event and cannot replace a submitted draft.

## Worker: read its task and propose an answer

With the worker's scoped token already provisioned in its environment:

```sh
wecom-chat-cli --socket /run/wecom-chat-agent/bridge.sock \
  --bot-id BOT_ID --deployment support task get

printf '%s' 'A concise answer based on approved task data.' | \
  wecom-chat-cli --socket /run/wecom-chat-agent/bridge.sock \
  --bot-id BOT_ID --deployment support reply propose --allow-writes

# Alternative: report a risk and decline this task, without blocking its author.
wecom-chat-cli --socket /run/wecom-chat-agent/bridge.sock \
  --bot-id BOT_ID --deployment support memory report \
  --reason secret_request --allow-writes
```

Worker commands have no `--event` or recipient selector. Use `--dry-run` to validate
without writing or sending. `--allow-writes` signals invocation intent and grants
no server privilege. A proposal in review mode does not send anything. A risk report
cancels a pending unsent draft; it cannot retract an already sent message.

In explicitly enabled template mode, use `reply propose --template TEMPLATE_ID
--allow-writes`; IDs are listed by `task get`. The broker sends the exact configured
literal. Free text, unknown IDs and substitutions are rejected.

## Operator: inspect and review

```sh
wecom-chat-cli --socket /run/wecom-chat-operator/bridge.sock \
  --bot-id BOT_ID --deployment support reply get --event EVENT_ID

wecom-chat-cli --socket /run/wecom-chat-operator/bridge.sock \
  --bot-id BOT_ID --deployment support reply approve \
  --event EVENT_ID --digest REVIEWED_DIGEST --yes --allow-writes
```

`reply reject` takes the same target/digest/confirmation flags. Rejection is terminal.
Review content as untrusted text; never render executable content or let it instruct
an approving agent. Do not implement a supervisor that automatically approves every
model-generated digest. After an interrupted call, inspect `reply get` or `event get`;
`sent`, `failed` and `uncertain` are terminal. Repeated approval cannot resend.

Additional operator controls: `task revoke --event ID --yes`,
`session reset --event ID --expected-epoch N --yes`, and
`memory set --user USER_ID --status blocked|quarantined|watch|clear --yes`.
Each mutation also requires `--allow-writes` unless previewed with `--dry-run`.
A session reset invalidates its old events/tasks; the next incoming event carries
its new epoch. Operator quarantine lasts one hour; blocking is indefinite.

## Migration and limitations

Version 1 policies are rejected. Replace `enable_replies` with an explicit version
2 `reply_mode`, add `task_ttl_seconds` (10..reply TTL) and `templates` (an object),
and review the new roles. `event list/get/watch` now require the operator socket;
`reply send` and `memory flag` no longer exist. There is no compatibility bypass.

Version 1 ledgers migrate to version 2 without deleting deduplication history,
operator memory or send outcomes. Legacy events retain epoch zero and cannot
receive new grants. Preserve a protected backup and do not roll back an updated
ledger to an older binary. Restarts invalidate outstanding grants and approvals.

Only this bot's live callbacks are observed; there is no tenant-wide chat archive,
missed-message backfill or exactly-once host execution. Quotes and non-text content
are suppressed; URLs are never fetched. Replies are final frames, not token streams.

Raw admitted text and drafts expire after 24 hours by default; bounded tombstones
remain. The 10,000-event/64-MiB cap stops intake instead of deleting replay history.
Long-term archival, high-volume operation, a concrete grok bot adapter and live
acceptance remain future work. The [release pipeline](docs/releasing.md) prepares
Linux/macOS binaries and a checksum-verified npm wrapper when a maintainer tags a release. See the [command reference](docs/cli/wecom-chat-cli.md)
and [companion Skill](skills/wecom-chat/SKILL.md).

## Contributing and related projects

Read [CONTRIBUTING.md](CONTRIBUTING.md), [security reporting](SECURITY.md) and the
[release guide](docs/releasing.md). Markdown is canonical; `make docs` generates
the searchable HTML site and command reference.

- [WeCom Calendar CLI](https://github.com/AngelMsger/wecom-calendar-cli): rolling calendar retrieval.
- [WeCom Channel CLI](https://github.com/AngelMsger/wecom-channel-cli): isolated channel observation and reviewed actions.
