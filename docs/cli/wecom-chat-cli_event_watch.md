## wecom-chat-cli event watch

Stream bounded NDJSON with resumable event cursors

```
wecom-chat-cli event watch [flags]
```

### Options

```
      --cursor string       Last successfully handled event cursor
      --duration duration   Total observation deadline, at most 1h (default 1m0s)
  -h, --help                help for watch
      --max-events int      Maximum delivered events (default 100)
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
