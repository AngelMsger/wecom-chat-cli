## wecom-chat-cli memory

Manage scoped sender risk records

```
wecom-chat-cli memory [flags]
```

### Options

```
  -h, --help   help for memory
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
* [wecom-chat-cli memory list](wecom-chat-cli_memory_list.md)	 - List active risk records through the operator socket
* [wecom-chat-cli memory report](wecom-chat-cli_memory_report.md)	 - Worker: report a fixed risk reason without changing user permissions
* [wecom-chat-cli memory set](wecom-chat-cli_memory_set.md)	 - Operator-only block, quarantine, watch or clear for an allowlisted user
