## wecom-chat-cli memory report

Worker: report a fixed risk reason without changing user permissions

```
wecom-chat-cli memory report [flags]
```

### Options

```
  -h, --help            help for report
      --reason string   prompt_injection, secret_request, destructive_request, abuse, other
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

* [wecom-chat-cli memory](wecom-chat-cli_memory.md)	 - Manage scoped sender risk records
