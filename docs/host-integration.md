# Integrating a host such as grok bot

The host is **grok bot**; its concrete SDK was not supplied. The CLI therefore
provides an enforced, framework-neutral task/review protocol. It does not create
host threads, provision sandboxes or claim a specific grok bot plugin integration.

## Authority split

A trusted deterministic supervisor/reviewer holds the operator socket. It may
observe admitted events, issue/revoke grants and reset session epochs. Only an
independent trusted review path may approve a free-text draft. A model worker
receives the worker socket and one short-lived capability; it cannot enumerate
messages or select an event. Never mount the operator socket into a worker.

The supervisor's routing code may parse raw data but must not turn all external
messages into the administrator agent's conversation. Reviewed replies are still
untrusted data, not instructions to the approving program. Never use a blanket
"approve the digest the model returned" loop.

## Session and thread routing

The broker derives `session_key` from deployment, bot ID, conversation type,
conversation ID and sender ID. A private chat is separate from a group chat;
users within the same group also have separate sessions. Text claiming another
identity cannot change these platform-derived coordinates.

Map `(session_key, session_epoch)` to the host thread ID. Route only the associated
history, summaries, retrieval context and tools into that thread. `session reset`
increments the broker's epoch using an expected-epoch check. The next incoming
event belongs to the new epoch; old tasks and drafts fail even if a worker still
has its token. Reset host history alongside this operation. The CLI cannot itself
clear the host's model memory or terminate its process.

Only one active task is issuable per session. A newer event waits until the older
task finishes, is rejected/revoked or expires. Once newer work has been issued,
older events in that epoch cannot be newly assigned. Different sessions can have
concurrent workers. Same-session serialization does not merge group participants'
history and does not grant private-data access to group replies.

## Supervisor sequence

1. Watch the operator inbox with a bounded deadline/count. Retain each record's
   trust envelope, `id`, `cursor`, `session_key` and `session_epoch`.
2. Durably enqueue and deduplicate by event ID. Check the reply deadline; do not
   route a stale connection/session event into privileged work.
3. Call `task issue --event ID`. Capture its secret token programmatically, without
   logging it or adding it to model context. Provision only that worker's tool
   executor with `WECOM_CHAT_TASK_TOKEN`. Do not share other workers' environments.
4. Start/reuse a sandboxed model context for the exact session/epoch. Give it only
   task read, reply proposal and risk-report capabilities. The worker calls `task
   get`; it does not receive a full inbox and cannot choose another event ID.
5. In default review mode, inspect the immutable proposal using `reply get`. A
   trusted reviewer evaluates accuracy, appropriateness, disclosure, recipient and
   risk report. Only then approve the exact digest. Expired/revoked/reset/changed
   tasks cannot be approved. An approval timeout is not a reason to widen authority.
6. Record the observed result and advance the host checkpoint after handling. A
   watch cursor acknowledges output, not model job completion. On an uncertain
   send, inspect the outcome and never manufacture another event to retry it.

If task issuance succeeds but token delivery is lost, `event get` reveals grant
metadata without disclosing the token again. Revoke and reissue before any draft
exists. Outstanding grants expire no later than the event's local reply deadline.
On broker restart, policy change or WebSocket reconnection they become unusable.
Reissuing an old connection's event is not a backfill or proactive-send mechanism.

## Worker API

Use `POST /v1/request` over the worker Unix socket. Supply the capability in the
`Authorization: Bearer ...` header, not JSON, argv or URL parameters. The CLI reads
it from its dedicated environment variable only for worker operations.

```json
{"op":"task.get","bot_id":"BOT_ID","deployment":"support"}
```

The result has a fixed trust envelope and contains `task_id`, `expires_at`,
`reply_mode`, `template_ids`, and an `event` with its own provenance envelope.
Only the authorized event is returned. No token/hash, remote callback request ID,
other user's content or general configuration is returned.

```json
{"op":"reply.propose","bot_id":"BOT_ID","deployment":"support","text":"A candidate answer","dry_run":true}
```

Omit `dry_run` only to persist the draft under existing task authorization. In
review mode this still sends nothing. The worker cannot provide `event_id`, user,
recipient, session, epoch, endpoint, approval or unrelated fields. A different
second draft conflicts; an identical retry returns its stored result.

```json
{"op":"memory.report","bot_id":"BOT_ID","deployment":"support","reason":"secret_request"}
```

A report is immutable and event-bound. It cancels that event's pending reply path
without changing the sender's access in other conversations or future events.
Only the operator can set quarantine/block/watch/clear memory. A report made after
sending is evidence only and cannot retract the message.

## Independent approval and templates

`reply get --event ID` is operator-only and exposes the full text, recipient,
session/epoch, expiration, task ID, digest, report and durable send status. Approval
requires the exact digest and operator socket; `--yes` is confirmation, not a
credential. The digest binds text, target, task, epoch, connection, expiry and policy.

Explicit `template` policy permits automatic sending only of configured literal
strings. The worker submits `template` instead of `text`; IDs come from `task get`.
There are no substitutions, arbitrary destinations or free-text fallbacks. The
operator must choose universally safe public templates. Selection quality remains
a model limitation, and a template cannot be revoked retroactively after sending.

## Host responsibilities that remain

The host must provision the actual sandbox, per-session model history, durable
job queue, reviewer and thread lifecycle. A stolen grant authorizes that exact task
until revocation/expiry; prevent cross-worker credential access. The CLI rejects
cross-task reads/writes even if the model guesses another event ID, but cannot
prevent a privileged host tool or another authenticated connector bypassing it.
