## wecom-chat-cli broker serve

Maintain the smart-bot connection and scoped worker/operator sockets

```
wecom-chat-cli broker serve [flags]
```

### Options

```
  -h, --help            help for serve
      --policy string   Absolute policy JSON path
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

* [wecom-chat-cli broker](wecom-chat-cli_broker.md)	 - Run the credential-holding operator service
