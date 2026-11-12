## wecom-chat-cli relay

Deliver pending references and track acceptance separately from processing

```
wecom-chat-cli relay [flags]
```

### Options

```
  -h, --help   help for relay
```

### Options inherited from parent commands

```
      --allow-writes        Permit this invocation's mutation; grants no server authority
      --bot-id string       Expected bot ID from the operator
      --deployment string   Expected deployment name from the operator
      --dry-run             Validate a write without state changes or remote sends
      --format string       Output: json or ndjson (trust fields cannot be removed) (default "json")
      --socket string       Absolute worker or trusted operator Unix socket path
```

### SEE ALSO

* [wecom-chat-cli](wecom-chat-cli.md)	 - Isolate agents from untrusted WeCom smart-bot conversations
* [wecom-chat-cli relay claim](wecom-chat-cli_relay_claim.md)	 - Atomically claim a webhook batch in supervisor-owned state before processing
* [wecom-chat-cli relay consume](wecom-chat-cli_relay_consume.md)	 - Record a claimed batch as processed or failed; never send chat replies
* [wecom-chat-cli relay resolve](wecom-chat-cli_relay_resolve.md)	 - Reconcile an unknown HTTP acceptance using verified host evidence
* [wecom-chat-cli relay run](wecom-chat-cli_relay_run.md)	 - Trusted service: deliver nonempty metadata batches without a model
* [wecom-chat-cli relay status](wecom-chat-cli_relay_status.md)	 - Read local delivery counts and timestamps; this does not probe process liveness
