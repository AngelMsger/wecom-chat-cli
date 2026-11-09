## wecom-chat-cli reply propose

Worker: submit stdin text for review, or select an approved literal template

```
wecom-chat-cli reply propose [flags]
```

### Options

```
  -h, --help              help for propose
      --template string   Policy-approved literal template ID; only in template mode
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
