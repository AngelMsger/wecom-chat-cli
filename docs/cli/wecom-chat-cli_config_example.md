## wecom-chat-cli config example

Print a deny-by-default operator policy

```
wecom-chat-cli config example [flags]
```

### Options

```
  -h, --help   help for example
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

* [wecom-chat-cli config](wecom-chat-cli_config.md)	 - Inspect operator policy
