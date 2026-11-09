## wecom-chat-cli task get

Worker: read only the task bound to WECOM_CHAT_TASK_TOKEN

```
wecom-chat-cli task get [flags]
```

### Options

```
  -h, --help   help for get
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
