# Assessment and scope

Checked against the public official CLI reference and smart-bot SDK on
2026-11-06. Service methods in the official CLI are dynamically discovered;
documentation inspection does not prove what every tenant's gateway exposes.

## Recommendation

Build this as a focused bridge and policy broker if untrusted employees need
reactive conversations with agents that otherwise hold powerful tools. Transport
alone is not a strong reason to maintain another CLI: the official SDK already
implements it. The durable value is independently enforced admission, provenance,
scoped task capabilities, independently reviewed replies, risk memory and a portable host interface.

If a host already provides equivalent bot transport, separate workers, least
privilege, per-sender state and a policy-controlled reply channel, an adapter
may be cheaper. Reassess the official CLI as it evolves. Official feature overlap
does not by itself replace this isolation boundary.

## Feasibility and evidence

- The official [Node SDK](https://github.com/WecomTeam/aibot-node-sdk) documents
  `wss://openws.work.weixin.qq.com`, subscription with Bot ID/Secret, pushed
  callbacks, heartbeats and replies using the callback's `req_id`.
- Its [connection manager](https://github.com/WecomTeam/aibot-node-sdk/blob/main/src/ws.ts)
  serializes replies per request, waits for acknowledgements, and stops reconnecting
  when displaced by another subscriber. This bridge follows those semantics.
- Its [message types](https://github.com/WecomTeam/aibot-node-sdk/blob/main/src/types/message.ts)
  distinguish sender ID, group/single conversation, message ID and optional quote.
- The [official CLI reference](https://github.com/WecomTeam/wecom-cli/blob/main/docs/cli-reference.md)
  describes dynamically discovered remote methods and gives `message aibot sessions
  list` as an example. It does not establish a persistent, isolated agent bridge.
  Calling this simply a "static message-list CLI" would overstate what was verified.
- Tencent's [ADP integration guide](https://cloud.tencent.com/document/product/1759/121473)
  describes smart-bot single conversations and group mentions via long connection.
  It is not permission to observe all employee conversations.

The transport is feasible without a public inbound endpoint. Delivery during
disconnection, tenant permissions, exact reply validity windows and compatibility
with existing bot integrations require live testing in the target tenant.
The default 120-second reply TTL is a conservative local policy, not a claim
about an official server deadline. The upstream can reject a shorter window.

## Safety assessment

Prompt injection detection is inherently incomplete. Obfuscated, indirect or
multi-turn attacks can pass a rule or model classifier. A safe architecture must
remain bounded when the model follows an attack:

1. The privileged supervisor owns policy, secrets and worker lifecycle.
2. Only a disposable low-privilege worker receives raw external messages.
3. The broker exposes single-event task reads, immutable reply drafts and risk reports,
   never shell/file access or arbitrary recipients.
4. Business actions needing authority must be introduced later as separately
   authorized narrow capabilities, outside conversational claims of identity.

Giving the same unsandboxed host agent a cautionary message is not isolation.
Similarly, output secret patterns cannot protect secrets already accessible
through another tool. Keep sensitive data out of the conversational worker.

## Family applicability

This is a new product, not a change to shared contracts. Existing checkouts were
inspected and remain unchanged. The workspace's `fx` executable was unavailable;
repository status was checked directly instead.

| Project | Applicability and inspected pattern | Required change |
| --- | --- | --- |
| Bitbucket | Kit output/errors/transport in app and pkg; no smart-bot service | None |
| Confluence | Same family adapters; no smart-bot service | None |
| Jenkins | Same family adapters; no smart-bot service | None |
| Jira | Same family adapters; no smart-bot service | None |
| OpenObserve | Same family adapters; no smart-bot service | None |
| Prometheus | Same family adapters; no smart-bot service | None |
| WeCom Calendar | Kit interfaces and local state precedent; calendar domain | None |
| WeCom Channel | Closest security reference: independent broker, sockets, immutable provenance, durable outcomes | None; safejson/securefs sources ported with MIT attribution |
| agent-cli-kit | Reuse pinned v0.9.0 output/errors/argv; no generic behavior changes | None |
| fx-devkit / workspace hub | Workspace registration is outside this repository | None |

Intentional prerelease differences: explicit socket/bot/deployment flags instead
of named contexts; broker-only environment credentials instead of worker login;
no raw/table/projection output; no automatic runtime update checks;
companion Skill is a checked-in guide, without an install/handshake command.
Repository, Pages, CI and release infrastructure follow the [family release pattern](releasing.md).
The kit's HTTP retry layer does not implement WebSockets; the domain transport
therefore owns subscription/reconnect, with no retry of remote writes.

Free-text bot replies require independent operator approval of the exact draft
digest. Unattended sends require explicit template policy and contain only reviewed
literal text, without substitutions. The bot sends in its own identity. No worker
can select an event, approve itself or change a sender's access permissions.

## Monitoring cost review (2026-11-10)

The family's [event-driven integration guide](https://github.com/AngelMsger/agent-clis-workspace/blob/9c88e4c/docs/event-driven-agent-integration.md)
sets the target: ordinary software owns collection, filtering, supervision and
reliable handoff; idle monitoring starts no model turns. Output, durable acceptance
and completed reasoning are separate stages. Prefer compact references and fetch
only the context needed for an accepted task. Preserve source coverage and resolve
ambiguous work instead of erasing cursors or automatically replaying actions.

Chat already had pushed intake, durable callback dedupe, bounded long polling and
isolated event grants. The local gaps were replaying history on every fresh watch,
returning full bodies and already assigned/closed records, and calling stdout
progress `delivered`. The pending-reference stream fixes those defaults. The host
still owns durable scheduling and actual model submission; no Grokbot integration
or measured end-to-end saving is claimed. See [host integration](host-integration.md).

| Project | Existing observation primitive | Applicability to this change |
| --- | --- | --- |
| WeCom Chat | Local callback ledger and operator watch | Apply quiet baseline, pending references and explicit emitted progress |
| WeCom Channel | Durable webhook relay (`internal/relay`) | Reference for handoff/idempotency; Chat keeps the local grant boundary |
| WeCom Calendar | ETag mirror sync (`internal/sync/sync.go`) | Same zero-idle-model target; no Chat feed or grant changes |
| Bitbucket | Bounded activity reads (`internal/app/activity_filter.go`) | Host observation responsibility applies; no local stream change |
| Confluence | Remote subscriptions (`internal/app/page_watch.go`) | A subscription is not an agent-job dispatcher; retain provider meaning |
| Jenkins | Byte-offset log follow (`internal/app/build_log.go`) | Streaming stdout is not a durable work queue; no Chat grant semantics |
| Jira | Bounded worklogs (`internal/app/worklog.go`) | Same separation of retrieval and model scheduling; no change here |
| OpenObserve | Timestamp query tail (`internal/app/search.go`) | Needs its own coverage/recovery contract; outside this product's fix |
| Prometheus | Metric queries (`internal/app/query.go`) | Meaningful alert transitions belong to an adapter, not Chat's pending feed |
| agent-cli-kit | Shared CLI libraries | Reuse current output/errors; no speculative shared relay extraction |

Only Chat is changed. Separate messages keep separate grants even within one
session; notification batching must not discard questions or outlive reply TTLs.
