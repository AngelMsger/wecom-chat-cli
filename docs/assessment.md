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
