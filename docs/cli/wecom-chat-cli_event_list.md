## wecom-chat-cli event list

Read one bounded page; reuse its cursor

```
wecom-chat-cli event list [flags]
```

### Options

```
      --cursor string        Cursor from a prior page or event
  -h, --help                 help for list
      --include-suppressed   Include redacted quarantined/suppressed events for operator inspection
      --limit int            Maximum events, 1..100 (default 50)
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
