## wecom-chat-cli reply

Propose immutable replies and independently review sends

```
wecom-chat-cli reply [flags]
```

### Options

```
  -h, --help   help for reply
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
* [wecom-chat-cli reply approve](wecom-chat-cli_reply_approve.md)	 - Operator: approve the exact immutable proposal digest
* [wecom-chat-cli reply get](wecom-chat-cli_reply_get.md)	 - Operator: inspect the exact draft, target, digest and outcome
* [wecom-chat-cli reply propose](wecom-chat-cli_reply_propose.md)	 - Worker: submit stdin text for review, or select an approved literal template
* [wecom-chat-cli reply reject](wecom-chat-cli_reply_reject.md)	 - Operator: reject the exact immutable proposal digest
