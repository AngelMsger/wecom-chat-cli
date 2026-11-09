## wecom-chat-cli session reset

Advance the session epoch and invalidate all previous tasks

```
wecom-chat-cli session reset [flags]
```

### Options

```
      --event string         An event identifying the session
      --expected-epoch int   Current session epoch from an event record
  -h, --help                 help for reset
      --yes                  Confirm the conversation reset
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

* [wecom-chat-cli session](wecom-chat-cli_session.md)	 - Operator-controlled conversation fences
