## wecom-chat-cli reply get

Operator: inspect the exact draft, target, digest and outcome

```
wecom-chat-cli reply get [flags]
```

### Options

```
      --event string   Event ID with a proposal
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

* [wecom-chat-cli reply](wecom-chat-cli_reply.md)	 - Propose immutable replies and independently review sends
