## wecom-chat-cli relay resolve

Reconcile an unknown HTTP acceptance using verified host evidence

```
wecom-chat-cli relay resolve [flags]
```

### Options

```
      --batch-id string   Exact unknown batch ID (required)
      --evidence string   Non-secret host run or operator reconciliation reference (required)
  -h, --help              help for resolve
      --outcome string    accepted or not_accepted, verified from host history (required)
      --state string      Private sender-owned relay state (required)
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
