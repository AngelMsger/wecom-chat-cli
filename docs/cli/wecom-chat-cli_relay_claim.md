## wecom-chat-cli relay claim

Atomically claim a webhook batch in supervisor-owned state before processing

```
wecom-chat-cli relay claim [flags]
```

### Options

```
      --batch-file string       Original relay batch JSON; - reads stdin (required)
      --consumer-state string   Private supervisor-owned claim file, one per bot/deployment/ledger (required)
  -h, --help                    help for claim
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

* [wecom-chat-cli relay](wecom-chat-cli_relay.md)	 - Deliver pending references and track acceptance separately from processing
