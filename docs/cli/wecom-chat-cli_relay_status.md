## wecom-chat-cli relay status

Read local delivery counts and timestamps; this does not probe process liveness

```
wecom-chat-cli relay status [flags]
```

### Options

```
      --batch-id string         Inspect a specific retained delivery or supervisor claim
      --consumer-state string   Private supervisor-owned processing state; mutually exclusive with --state
  -h, --help                    help for status
      --state string            Private sender-owned relay state
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
