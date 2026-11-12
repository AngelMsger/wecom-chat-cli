# Contributing

Use Go 1.25 or later. Layers are cmd -> internal/{app,broker,state,config} and
pkg/{aibot,errors,safejson,transport}. The separate internal/relay service ports
Channel delivery and claim persistence, with Chat metadata and deadline rules.
It never adds webhook or grant-dispatch hooks to the credential broker. The WebSocket adapter follows the official SDK's
wire contract. Shared CLI behavior comes from agent-cli-kit.

Run `GOWORK=off make lint test e2e build docs-check site-check release-tests`,
`npm test --prefix build/npm` and `make vuln`. Security tests must cover denial,
socket authority, scope, duplicate callbacks, uncertain writes, restart, and
memory poisoning. No live messages are sent by tests. `make docs` regenerates
the command reference. Read docs/security.md before changing authorization.

Use a branch and pull request after repository bootstrap; main requires Linux/macOS
CI and Linux isolation. Never bypass those protections. Run `make docs` after any
command or narrative change, keeping Markdown canonical. The static site has no
remote runtime assets. Follow [releasing](docs/releasing.md) for tag-driven builds,
Pages configuration and the npm publisher setup. Do not tag merely
to test a release. See [SECURITY.md](SECURITY.md) for private reporting.
