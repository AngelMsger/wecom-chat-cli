# WeCom Chat CLI

Built for agents. Designed for humans.

The npm package installs the native WeCom smart-bot CLI for Linux/macOS on amd64
or arm64. The installer requires the matching GitHub Release's SHA-256 checksum.
The runtime launcher never downloads code implicitly.

Install the released package in a trusted setup session:

```sh
npm install -g @angelmsger/wecom-chat-cli@0.4.0
wecom-chat-cli auth guide
wecom-chat-cli config example
```

Version 0.4.0 adds durable metadata webhook delivery and supervisor-owned claims.
A Grok HTTP acceptance is not processing completion; the trusted host still owns
single-event task provisioning and independent reply review.

Provision the broker, isolated worker and separate operator review path using the
[operator guide](https://angelmsger.github.io/wecom-chat-cli/guide.html).
The broker alone receives bot credentials. Conversation workers receive one
short-lived task grant; they cannot select a recipient or approve their own draft.

The published package includes the matching `SKILL.md`. Install that guide in the
host's skill directory and reload the worker context after upgrading. There is no
`skill install` command in this CLI. No host adapter, service or OS account is
installed automatically. Windows is not supported.
