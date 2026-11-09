# Visual design

WeCom Chat CLI uses a light blue/white documentation surface, navy code panels,
readable system typography and the family slogan, "Built for agents. Designed for
humans." Its original chat/code mark is not an official Tencent logo.

Calendar and Channel provide the family references for naming, navigation and
static documentation. Chat has its own conversation/task illustration and worker
request explorer. The explorer uses local synthetic data and clearly labels the
simplified output; it makes no network requests and carries no credentials.

The landing page is `docs/index.html`; local assets live in `docs/assets/`.
`cmd/gen-docs` renders the shared page chrome, narrative Markdown and searchable
command reference. Run `make docs docs-check site-check`. Keep the existing
per-command Markdown URLs stable. Canonical URLs and sitemap use the repository's
GitHub Pages project path. No remote scripts, fonts or image hosts are required.

Check desktop and mobile widths, keyboard focus, the copy button, task scenarios,
command search and browser errors before publishing a layout change. The
prerelease banner and source-build instructions remain until a release actually
exists. Do not imply that prompt-injection screening guarantees safe answers.
