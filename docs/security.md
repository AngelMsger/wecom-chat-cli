# Security and deployment

## Required isolation

The CLI constrains what goes through its broker. It cannot stop an unsandboxed
host agent from using a shell, filesystem, browser, MCP connector or another
authenticated WeCom tool. Run the complete conversation worker and its tool
executor under a separate OS identity or in an unprivileged container/VM. A
different prompt or process with the administrator's UID is not isolation.

The worker receives only its own short-lived task token; never place all workers'
grants in a shared environment or mount. Isolate workers from each other's process
environments, memory and files as well, using distinct identities or containers;
separate threads under a shared unrestricted UID do not provide that boundary.
The worker must have no host home, private repository, cloud credentials, SSH
agent, OS keychain, Docker socket, sudo, broker environment, operator socket,
broker state, or writable policy mount. Deny general outbound network and
privilege escalation, enforce resource/deadline quotas, and provide only public
or explicitly authorized conversation data. Model inference can be mediated by
a narrow host-controlled gateway. Do not expose a general service token to the
model's tool executor. Other authenticated tools can bypass this broker.

Only trusted deterministic supervisor/reviewer code or the human operator may use
the operator socket. A model must not receive it merely to discover events or
approve its own draft. Never auto-approve every submitted digest.

Keep the privileged supervisor's administrator chat and memory separate. Give
each sender/conversation its own worker context identified by `session_key` plus
`session_epoch`, and reset disposable execution state between sessions. In group chats the session key
also includes the sender; a response still appears in the group and must not
contain private information. Receiving a group mention does not authorize access
to a user's other messages or resources.

## Authority

| Actor | Capabilities | Excluded |
| --- | --- | --- |
| WeCom sender | Send external content within allowed bot conversations | Operator identity, host commands, policy changes |
| Agent worker | Read its granted event; propose one draft or a literal template; report risk | Inbox enumeration, choosing event/recipient, self-approval, user memory changes, other grants, bot credentials |
| Broker | Fixed TLS endpoint, private ledger, two local interfaces | Shell execution, arbitrary fetch/download/upload, tool dispatch |
| Trusted supervisor/reviewer/operator | Observe inbox; issue/revoke task grants; reset sessions; approve exact drafts; manage user memory | Authority inferred from external content or a model selecting its own privilege |

All users, including allowlisted employees and people claiming to be the owner,
remain untrusted content authors. Transport authentication verifies the source
channel and event metadata; it does not convert message text into instructions.

## Enforced controls and limits

| Threat | Control | Limit |
| --- | --- | --- |
| Prompt injection | Fixed provenance envelope; bounded heuristic screen; no command executor | Classifiers can be bypassed; the worker sandbox must remain safe on a false negative |
| Secret disclosure | Worker has no private data; broker suppresses reflected bot Secret; outbound credential-pattern check | Encoded or semantic leaks are not reliably detected; do not give the worker sensitive data |
| Memory poisoning | Worker reports have a fixed reason and task-bound target; only operator changes permissions | A false report can decline its own task, but cannot silence the user in future conversations |
| Misrouted replies | Scoped token, no worker event selector, stored callback target, session epoch and connection binding | The supervisor must provision each token to the correct isolated worker |
| Wrong or fabricated answer | Default immutable draft and independent review; optional fixed literal templates | Neither pattern checks nor a digest verify truth; reviewer/template quality remains a responsibility |
| Duplicate sends | Durable `sending` before I/O; crash recovery to `uncertain`; no automatic replay | Exactly-once remote delivery is not promised without upstream idempotency |
| Privilege escalation | Capability-only worker operations; separate operator listener; immutable digest approval | Same-user access to operator resources defeats the boundary |
| Path and URL attacks | No file/media or arbitrary-URL interface; fixed server endpoint | Broker OS/root/proxy administrators remain trusted |
| Terminal spoofing | JSON-only external records, no raw/table/field removal; controls intercepted | A downstream UI must keep text as data and must not fetch links |
| Denial of service | Frame/body/queue/concurrency/byte/rate/ledger bounds | An allowed sender can exhaust bounded capacity; host resource limits and future archival remain necessary |

Heuristics cover conspicuous Chinese/English instruction override, secret requests
and destructive command requests, with limited full-width/format-character handling.
They are deliberately neither a complete policy engine nor proof of benign intent.
An optional model classifier could improve recall later, but may only narrow
capabilities, never grant trust. Treat its conclusions as untrusted suggestions.

## Linux installation reference

Create dedicated `wecom-chat` service and worker accounts and a shared
`wecom-chat-agents` group. Only intended workers belong to that group. The broker
uses that primary group; the operator administers it through an external trusted
mechanism the worker cannot invoke. Provision this layout:

| Path | Owner/group | Mode |
| --- | --- | --- |
| `/etc/wecom-chat/` | root/root | 0755 |
| `/etc/wecom-chat/broker.json` | root/root | 0644 |
| `/etc/wecom-chat/secrets.env` | root/root | 0600 |
| `/var/lib/wecom-chat/` | wecom-chat/wecom-chat-agents | 0700 |
| `/run/wecom-chat-agent/` | wecom-chat/wecom-chat-agents | 0750 |
| `/run/wecom-chat-agent/bridge.sock` | wecom-chat/wecom-chat-agents | 0660 (created by service) |
| `/run/wecom-chat-operator/` | wecom-chat/wecom-chat-agents | 0700 |
| `/run/wecom-chat-operator/bridge.sock` | wecom-chat/wecom-chat-agents | 0600 (created by service) |

The example socket filename remains `bridge.sock` because it identifies the local
broker channel, not another public CLI. Every path ancestor must be root/service
owned and not group/world-writable (except trusted sticky temporary roots for
tests). Symlinked ancestors and unexpected file hard links are rejected.

Review [the service unit](../deploy/wecom-chat.service) and
[runtime directory configuration](../deploy/wecom-chat.tmpfiles.conf) before
installation. They are references, not automatically deployed. Install a reviewed
binary at `/usr/local/bin/wecom-chat-cli`, provision the policy and broker-only
environment, validate, then deliberately start the service. Automatic restart is
disabled to avoid fighting another bot subscriber after displacement.

If an unclean exit leaves sockets behind, verify no service is active and preserve
the ledger before removing only the stale socket entries. The CLI refuses to
unlink an existing endpoint automatically. Never remove the ledger to clear a
send failure or full capacity. Keep logs free of raw content and credentials.
Use encrypted storage and appropriate backup retention for personal data.

## Live acceptance gates

Offline tests cannot attest a real host's sandbox or a tenant's API rollout.
Before unattended use, verify with a dedicated test bot:

1. Intended single-chat and group-mention callbacks carry the expected IDs; other
   users/groups cannot enter the worker. This is not an enterprise chat archive.
2. A normal draft stays unsent until independent exact-digest approval, then works
   within the local deadline with a final response and acknowledgement. In template
   mode only the reviewed literal is sent. Do not silently widen the deadline.
3. Duplicate callbacks, disconnects and process interruption do not resend replies;
   coverage gaps are visible and a competing subscriber stops the service.
4. Actual worker identity cannot open the operator socket, state, credentials or
   privileged host paths, or use alternate authenticated connectors.
5. Malicious content is treated as data even when heuristics miss it; each worker
   remains unable to run privileged commands or retrieve secrets.
6. A worker grant cannot read another event, select its recipient, enumerate the
   inbox, issue grants or approve drafts. Old grants fail after revoke/reset.
7. A model risk report cannot modify user memory or suppress future normal messages;
   only operator quarantine/block can do so. Audit token-free output and storage.

Task grants authorize only one event and expire within its reply deadline.
Revocation, user blocks, reset, policy changes and reconnect/restart invalidate
access. They are bearer credentials: a stolen grant can still read/propose/report
for that task. The broker stores only a hash. Protect the one-time operator issue
response and do not put it in model context, diagnostics or shell arguments.

Use an explicitly authorized test conversation for live sends. No such send was
performed during this implementation.
