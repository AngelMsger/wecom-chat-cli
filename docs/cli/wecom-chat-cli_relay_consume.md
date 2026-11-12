## wecom-chat-cli relay consume

Record a claimed batch as processed or failed; never send chat replies

```
wecom-chat-cli relay consume [flags]
```

### Options

```
      --batch-id string         Batch ID returned by relay claim (required)
      --claim-id string         Claim token returned only to the successful claimant (required)
      --consumer-state string   Private supervisor-owned claim file (required)
  -h, --help                    help for consume
      --result string           processed or failed (required); neither permits automatic replay
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
