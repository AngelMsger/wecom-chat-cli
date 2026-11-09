## wecom-chat-cli event get

Inspect one event and its durable reply outcome

```
wecom-chat-cli event get [flags]
```

### Options

```
      --event string   Event ID from list/watch or a reply result
  -h, --help           help for get
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

* [wecom-chat-cli event](wecom-chat-cli_event.md)	 - Operator-only event discovery and monitoring
