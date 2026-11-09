## wecom-chat-cli task issue

Operator: issue a short-lived grant; stdout contains a secret token

```
wecom-chat-cli task issue [flags]
```

### Options

```
      --event string   Admitted event ID from event list/watch
  -h, --help           help for issue
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

* [wecom-chat-cli task](wecom-chat-cli_task.md)	 - Issue or consume single-event worker grants
