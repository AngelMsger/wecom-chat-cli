## wecom-chat-cli memory set

Operator-only block, quarantine, watch or clear for an allowlisted user

```
wecom-chat-cli memory set [flags]
```

### Options

```
  -h, --help            help for set
      --reason string   Fixed reason code (default "other")
      --status string   blocked, quarantined, watch or clear
      --user string     Stable user ID from an event or reviewed operator policy
      --yes             Confirm the selected user's policy change
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
