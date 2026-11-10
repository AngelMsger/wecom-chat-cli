## wecom-chat-cli event watch

Stream new pending event references; explicit replay or cursor resumes history

### Synopsis

A trusted ordinary supervisor runs this bounded stream, never a scheduled model turn.
By default start at the current ledger head and emit only metadata for live, unassigned
events. Use --replay for retained pending work or --cursor to resume. Output is not
durable host acceptance: enqueue and deduplicate before saving progress or starting a model.

```
wecom-chat-cli event watch [flags]
```

### Options

```
      --cursor string       Watch cursor saved after durable host enqueue; not a model-completion checkpoint
      --duration duration   Total observation deadline, at most 1h (default 1m0s)
  -h, --help                help for watch
      --max-events int      Maximum emitted event references (default 100)
      --replay              Explicitly replay retained live, unassigned events instead of starting now
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
