# WeCom Chat CLI

Built for agents. Designed for humans.

The npm package installs the native WeCom smart-bot CLI for Linux/macOS on amd64
or arm64. The installer requires the matching GitHub Release's SHA-256 checksum.
The runtime launcher never downloads code implicitly.

**Prerelease package metadata only: no npm version has been published yet.**
Build from [source](https://github.com/AngelMsger/wecom-chat-cli) until the first
release is announced. Once published, install the exact released package version
in a trusted setup session, then run `wecom-chat-cli auth guide` and
`wecom-chat-cli config example`.

Provision the broker, isolated worker and separate operator review path using the
[operator guide](https://angelmsger.github.io/wecom-chat-cli/guide.html).
The broker alone receives bot credentials. Conversation workers receive one
short-lived task grant; they cannot select a recipient or approve their own draft.

The published package includes the matching `SKILL.md`. Install that guide in the
host's skill directory and reload the worker context after upgrading. There is no
`skill install` command in this CLI. No host adapter, service or OS account is
installed automatically. Windows is not supported.
