# Integrating a host such as grok bot

The CLI supports Grok routine webhook notifications through a separate trusted
`relay run` process, alongside the existing pull-based supervisor protocol.
The host must still provide isolated workers, single-event token provisioning,
thread history and independent review. A webhook does not provide those capabilities.

## What starts a model task

"Wake the model" means that ordinary host code submits one eligible job to the
host's model runtime. It does not mean the CLI calls an LLM or can start Grokbot
through an unspecified API. The host remains in control. Its trusted supervisor
starts the CLI stream, waits without model inference, and invokes a worker only
after durable enqueue, deduplication and successful task issuance.

```mermaid
sequenceDiagram
    participant wecom as WeCom
    participant broker as Chat CLI broker
    participant supervisor as Host supervisor (ordinary code)
    participant worker as Isolated model worker
    supervisor->>broker: event watch over operator socket
    wecom->>broker: WebSocket callback
    broker->>broker: Admit, deduplicate and commit ledger
    broker-->>supervisor: Pending event reference
    supervisor->>supervisor: Atomically deduplicate, enqueue and save cursor
    supervisor->>broker: task issue for queued event
    broker-->>supervisor: One short-lived grant
    supervisor->>worker: Submit model job and provision tool credential
    worker->>broker: task get over worker socket
    broker-->>worker: Only the authorized message
    worker->>broker: reply propose
    supervisor->>broker: Independent review and exact-digest approval
    broker->>wecom: Reply to the original callback
```

The supervisor/job-submission arrows describe the host integration contract,
not an implemented Grokbot SDK adapter. Verify that the host can durably accept
a job from ordinary code without first invoking a model. A cron prompt asking
the model to run `event watch`, or a model repeatedly checking an idle process,
still spends model turns. Zero idle stdout alone does not prove zero inference.

Choose one collector for a deployment: the watch-plus-host-inbox sequence below,
or the CLI's durable relay. Both run outside model turns. The broker has no
webhook endpoint configuration or arbitrary command execution.

## Low-cost observation contract

`event watch` starts at the current ledger head by default, without replaying
existing records. `--replay` explicitly includes retained pending work;
`--cursor` resumes the exact watch feed and cannot be combined with `--replay`.
Every bounded invocation must resume the last durably enqueued cursor, including
the terminal cursor from an idle invocation. Repeatedly starting without a cursor
creates a new quiet baseline and can skip arrivals between processes.

Only live admitted events with no existing task, proposal, report or send outcome
are emitted. Expired, old-connection, old-session, blocked, suppressed and
superseded events do not become model work. Existing/revoked grants are recovered
explicitly through operator audit reads, never automatically reissued by a watch.
Pending later messages in a busy session remain visible: queue them in sequence,
retry `task issue` in ordinary code within their deadlines, and start no model
until issuance succeeds. Discovery is not a reservation; policy or deadlines may
change before issuance. Distinct messages are not collapsed into one reply grant.

Each NDJSON record keeps the fixed trust envelope and a versioned reference:
`ledger_id`, `id`, `sequence`, `cursor`, `bot_id`, `deployment`, `session_key`,
`session_epoch`, `received_at`, `reply_expires_at`, `content_hash`, `kind`, `view`
and `coverage`. It contains no message body, task credential, callback request ID
or connection generation. The worker obtains content with `task get`; operator
`event list/get` remain bounded full audit reads. Filtered wakeups continue waiting
within the original long-poll deadline; idle polls emit no records.
Long polls fit within the CLI's remaining duration, reserving 100 ms for response
delivery. A bounded watch may finish slightly early so its terminal cursor can
include filtered arrivals even during a short resumed invocation. A transport
failure can still require replay from the last saved cursor; never infer progress
past a reference that was not emitted and durably enqueued.

The terminal stderr notice uses `emitted` and `checkpoint_stage: emitted`.
It reports bytes written to stdout, not durable handoff or completed inference.
Do not inject this notice, connection logs or health probes into model context.
The watch itself has no host acknowledgment or durable inbox; the separate relay
owns those local collection/delivery checkpoints when selected.

## Durable webhook delivery

This implementation ports Channel v0.4.0's delivery and claim machinery. The
[Grok contract review](https://github.com/AngelMsger/wecom-channel-cli/blob/main/docs/research.md#grok-bot-routine-webhook-review--2026-11-12)
records the official [routine webhook documentation](https://cursor.com/help/grok-bot/routines.md)
reviewed on 2026-11-12: POST JSON to the routine's fixed URL with its sender key
as a Bearer credential; exactly HTTP 200 means accepted and started, not completed.
It does not promise deduplication of `Idempotency-Key` or a completion callback.
No live routine was invoked to validate this port.

| Mode | Success | Recovery responsibility |
| --- | --- | --- |
| `explicit_ack` (default) | 2xx with exactly `{"ack":"BATCH_ID"}` | Authenticated receiver atomically deduplicates and durably enqueues before ACK; retry the same immutable batch |
| `http_accepted` (opt-in) | Exactly HTTP 200, without reading a completion body | Sender saves acceptance receipt; network loss/cancellation or leftover `sending` becomes `unknown` and stops pending host-history reconciliation |

Other HTTP responses in the documented Grok contract mean no run started.
Only transient rejection codes receive the configured bounded retry; redirects,
authentication and protocol errors stop. `Idempotency-Key` is correlation evidence,
not a receiver guarantee. The transport cannot silently replay a POST. Terminal
errors and retry budgets persist across process restart.

Provision private existing directories and protected JSON from
[relay.example.json](../deploy/relay.example.json) or
[grok.example.json](../deploy/grok.example.json). Replace the example URL with the
operator's actual routine URL. Inject the configured `token_env` through a protected
service environment, never argv, payloads or instructions. Use an ordinary OS
supervisor with [the service example](../deploy/wecom-chat-relay.service); finite
sessions resume the same state. Do not turn a terminal exit into a restart loop.

```sh
wecom-chat-cli --socket /run/wecom-chat-operator/bridge.sock \
  --bot-id BOT_ID --deployment support relay run \
  --config /etc/wecom-chat-relay/relay.json \
  --state /var/lib/wecom-chat-relay/state.json --duration 24h --dry-run
# Run the same command with --allow-writes instead of --dry-run after provisioning.
wecom-chat-cli relay status --state /var/lib/wecom-chat-relay/state.json
```

Dry-run is offline: no token lookup, broker requests, state writes or locks.
`--allow-writes` is local write intent, not additional broker authority. Fresh
state starts quietly at the current head; `--replay` applies only at creation.
Chat requires durable `--state`; it intentionally omits Channel's lossy ephemeral
mode for pending conversation work. Subsequent runs use the saved opaque cursor. Destination, limits, scope or policy
changes require reconciliation; do not delete the queue or restart with replay.
The watch gains additive page metadata for binding/progress, with unchanged
existing reference and cursor formats.

| Configuration | Default / bounds | Meaning |
| --- | --- | --- |
| `batch_window_seconds` | 1 / 1–300 | Maximum wait from the oldest pending reference; full batches seal early |
| `batch_size` | 50 / 1–100 | Maximum references per batch |
| `queue_limit` | 1000 / batch size–10000 | Pending plus inflight references; full queues stop collection, not cursor safety |
| `timeout_seconds` | 10 / 1–30 | One POST deadline |
| `retry_limit` | 0 / 0–20 | Extra delivery/source attempts, persisted across restarts |
| `retry_base_seconds` | 5 / 1–300 | Exponential backoff, capped at five minutes |
| `min_delivery_interval_seconds` | 0 / 0–3600 | Persisted spacing between POST starts, including retries |

This is bounded batching plus pacing, not a sliding quiet-period timer. Grouping
uses `(session_key, session_epoch)` and retains every message ID in order; it does
not merge requests or grant one worker several messages. Broker admission filters
remain authoritative. Sender collection/first-send checks and supervisor claims
skip expired references. Attempted payloads stay immutable even if their deadlines
pass, including after an explicit retry-budget reset. A separate lifetime attempt
marker prevents expiry from changing the original batch or its deduplication key.
The trusted supervisor still rechecks eligibility through `task issue` before
starting a conversation worker; reconnects, resets or earlier assignments can make
a queued reference stale.

Chat's default reply deadline is 120 seconds. The Grok example uses a one-second
window and five-second spacing as an illustration, not a throughput guarantee.
Tune against actual host quotas, shared account usage and reply latency; a backlog
can expire before a grant is issued. Do not copy Channel's 30/60-second example
without checking this budget. If the host cannot issue isolated single-event grants,
use direct webhooks only as metadata notifications, not an autonomous reply service.

### Supervisor claims and processing

The wire batch is `{version,id,identity,coverage,threads}`. Each thread has
`session_key`, `session_epoch` and `events`; each event preserves its `trust` and
metadata-only `data`. Authenticate the inbound request and validate its expected
source before using a private supervisor-owned file:

```sh
wecom-chat-cli relay claim --consumer-state /var/lib/chat-supervisor/claims.json \
  --batch-file - --allow-writes < /var/lib/chat-supervisor/incoming.json
```

Only `claimed: true` returns new, unexpired `work` plus a `claim_id`. Identical
batches or events in overlapping batches do not return work again; changed
metadata under an existing ID conflicts. An entirely stale new batch reports
`expired`. Claims retain work for recovery and never expire automatically.
Do not submit the filtered `work` as a new incoming batch. The trusted supervisor
queues each reference, preserves per-session ordering, issues each event's grant
and provisions it only to that worker. A busy session remains queued until
issuance succeeds or its deadline expires. This command does not implement the
host's job scheduler or authenticate the webhook by itself.

```sh
wecom-chat-cli relay consume --consumer-state /var/lib/chat-supervisor/claims.json \
  --batch-id BATCH_ID --claim-id CLAIM_ID --result processed --allow-writes
wecom-chat-cli relay status --consumer-state /var/lib/chat-supervisor/claims.json \
  --batch-id BATCH_ID
```

Use `processed` only after the supervisor durably records outcomes for its work;
`failed` is terminal and never authorizes replay. Status hides claim tokens.
A lost claim response may have committed: inspect the protected claim and host
job record rather than deleting claims. A batch accepted by the host is not a
completed job. Receiver dedupe before model submission prevents duplicate jobs;
claims inside an already-started routine cannot undo a paid wakeup.

### Reconcile delivery outcomes

Stop the sender and inspect its exact saved batch plus the host run history:

```sh
wecom-chat-cli relay status --state /var/lib/wecom-chat-relay/state.json --batch-id BATCH_ID
wecom-chat-cli relay resolve --state /var/lib/wecom-chat-relay/state.json \
  --batch-id BATCH_ID --outcome accepted --evidence VERIFIED_RUN_REFERENCE --allow-writes
```

Use `not_accepted` only with host evidence that no run started; absence of a claim
is insufficient. After verified non-acceptance, resume with the original
configuration/state and its remaining retry budget. Use `relay run --retry-failed`
only after the budget is exhausted or a terminal rejection is repaired. That flag
cannot clear an unknown outcome. `accepted` preserves the original batch in a receipt.

Sender and consumer snapshots are capped at 16 MiB; accepted receipts and batch
claims at 10000, event dedupe at 20000. Capacity stops safely, without deleting
unacknowledged responsibility. Archive only after reconciling pending work and the
possible replay lifetime. Private files must be mode 0600 with trusted ancestors;
symlinks and hard links are rejected. Files do not prove a service is alive. Health
monitoring, backups and a supported supervisor remain ordinary host responsibilities.

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

1. Run the operator watch in ordinary supervised code with a bounded deadline/count.
   Retain each reference and its trust envelope. Use a saved watch cursor on
   subsequent invocations; select the first quiet baseline or replay deliberately.
2. In one host transaction, insert the unique `(deployment, bot_id, ledger_id, id)`
   job and save the input cursor. An identical duplicate returns the existing job,
   without starting a second model; a changed content hash conflicts. Bind host
   state to the broker scope, watch feed and intended runtime destination. A full
   queue applies backpressure or stops consumption without advancing progress.
   Check the reply deadline; stale work is recorded as expired, not run through
   a model. Retain dedupe evidence across the possible replay lifetime.
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
6. Persist the job result separately from the enqueue cursor. Use durable job IDs,
   leases/fencing and result reuse in the host runtime. A crash after inference
   starts but before its result commits has an ambiguous outcome: reconcile job
   status instead of blindly running another model. On an uncertain send, inspect
   the broker outcome and never manufacture another event to retry it.

If task issuance succeeds but token delivery is lost, `event get` reveals grant
metadata without disclosing the token again. Revoke and reissue before any draft
exists. Outstanding grants expire no later than the event's local reply deadline.
On broker restart, policy change or WebSocket reconnection they become unusable.
Reissuing an old connection's event is not a backfill or proactive-send mechanism.

Do not revoke and reissue automatically merely because a worker is slow or its
submission response was lost. First reconcile the durable host job. If the host
has no idempotent job submission/status API, document the unresolved execution
window and keep ambiguous work for operator recovery. Batch enqueue transactions
or scheduler notifications if useful, while preserving every message ID, session
order and short reply deadline; Chat does not coalesce separate user requests.

## Migration and acceptance

Version 0.3.0 changes the watch from 0.2.x full history to pending references.
Audit/list and legacy watch cursors are intentionally rejected by the new feed;
do not translate them by decoding opaque cursor bytes. Preserve the old cursor,
host inbox, task grants and outcomes, stop the old trigger, then reconcile through
operator `event list/get`. Select a quiet baseline or `--replay` of still-live
unassigned work under the same host event dedupe. Never delete the ledger to reset
a cursor. Replaying does not resurrect expired grants or disconnected callbacks.

Test an idle interval and entirely filtered traffic for zero host job submissions
and zero model invocations. Test duplicate callbacks, replay, process restart,
lost output, crashes on both sides of host enqueue and job submission, busy
sessions, queue saturation, expired work and policy/ledger changes. Count source
events, emitted references, accepted jobs, model invocations and expired/ambiguous
jobs separately. Broker/CLI tests establish the local protocol; only a real host
adapter test can establish Grokbot behavior or a token-saving percentage.

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
