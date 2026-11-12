## wecom-chat-cli relay run

Trusted service: deliver nonempty metadata batches without a model

### Synopsis

Collect pending references over the operator socket into a durable private queue.
Empty scans send nothing. explicit_ack requires an exact batch acknowledgment;
http_accepted accepts only HTTP 200, never processing completion. Unknown outcomes
stop until relay resolve. No grants, reply approvals or message bodies are sent.
The receiver must use a trusted supervisor to issue one grant per live event.

```
wecom-chat-cli relay run [flags]
```

### Options

```
      --config string       Private fixed-destination relay JSON (required)
      --duration duration   Maximum service session duration, up to 24h; resume the same state (default 5m0s)
  -h, --help                help for run
      --interval duration   Minimum local broker scan interval, 1s..1m (default 1s)
      --max-batches int     Maximum acknowledged or HTTP-accepted batches in this session (default 1000)
      --replay              Replay retained pending work only when creating new state; default starts quietly now
      --retry-failed        Explicitly reset retry budgets after repair; never replays an unknown outcome
      --state string        Durable JSON queue in an existing private service-owned directory (required)
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

* [wecom-chat-cli relay](wecom-chat-cli_relay.md)	 - Deliver pending references and track acceptance separately from processing
