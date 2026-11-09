## wecom-chat-cli reply reject

Operator: reject the exact immutable proposal digest

```
wecom-chat-cli reply reject [flags]
```

### Options

```
      --digest string   Exact digest returned by reply get
      --event string    Reviewed event ID
  -h, --help            help for reject
      --yes             Confirm the reviewed proposal action
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
