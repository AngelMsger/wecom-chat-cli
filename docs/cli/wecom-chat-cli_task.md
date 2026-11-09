## wecom-chat-cli task

Issue or consume single-event worker grants

```
wecom-chat-cli task [flags]
```

### Options

```
  -h, --help   help for task
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

* [wecom-chat-cli](wecom-chat-cli.md)	 - Isolate agents from untrusted WeCom smart-bot conversations
* [wecom-chat-cli task get](wecom-chat-cli_task_get.md)	 - Worker: read only the task bound to WECOM_CHAT_TASK_TOKEN
* [wecom-chat-cli task issue](wecom-chat-cli_task_issue.md)	 - Operator: issue a short-lived grant; stdout contains a secret token
* [wecom-chat-cli task revoke](wecom-chat-cli_task_revoke.md)	 - Operator: revoke a task grant and prevent pending sends
