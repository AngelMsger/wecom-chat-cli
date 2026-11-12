## wecom-chat-cli

Isolate agents from untrusted WeCom smart-bot conversations

```
wecom-chat-cli [flags]
```

### Options

```
      --allow-writes        Permit this invocation's mutation; grants no server authority
      --bot-id string       Expected bot ID from the operator
      --deployment string   Expected deployment name from the operator
      --dry-run             Validate a write without state changes or remote sends
      --format string       Output: json or ndjson (trust fields cannot be removed) (default "json")
  -h, --help                help for wecom-chat-cli
      --socket string       Absolute worker or trusted operator Unix socket path
```

### SEE ALSO

* [wecom-chat-cli auth](wecom-chat-cli_auth.md)	 - Read credential provisioning guidance
* [wecom-chat-cli broker](wecom-chat-cli_broker.md)	 - Run the credential-holding operator service
* [wecom-chat-cli config](wecom-chat-cli_config.md)	 - Inspect operator policy
* [wecom-chat-cli event](wecom-chat-cli_event.md)	 - Operator-only event discovery and monitoring
* [wecom-chat-cli memory](wecom-chat-cli_memory.md)	 - Manage scoped sender risk records
* [wecom-chat-cli relay](wecom-chat-cli_relay.md)	 - Deliver pending references and track acceptance separately from processing
* [wecom-chat-cli reply](wecom-chat-cli_reply.md)	 - Propose immutable replies and independently review sends
* [wecom-chat-cli session](wecom-chat-cli_session.md)	 - Operator-controlled conversation fences
* [wecom-chat-cli status](wecom-chat-cli_status.md)	 - Inspect broker connectivity and scope
* [wecom-chat-cli task](wecom-chat-cli_task.md)	 - Issue or consume single-event worker grants
* [wecom-chat-cli version](wecom-chat-cli_version.md)	 - Show the local CLI version
