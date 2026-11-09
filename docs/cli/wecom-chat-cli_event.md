## wecom-chat-cli event

Operator-only event discovery and monitoring

```
wecom-chat-cli event [flags]
```

### Options

```
  -h, --help   help for event
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
* [wecom-chat-cli event get](wecom-chat-cli_event_get.md)	 - Inspect one event and its durable reply outcome
* [wecom-chat-cli event list](wecom-chat-cli_event_list.md)	 - Read one bounded page; reuse its cursor
* [wecom-chat-cli event watch](wecom-chat-cli_event_watch.md)	 - Stream bounded NDJSON with resumable event cursors
