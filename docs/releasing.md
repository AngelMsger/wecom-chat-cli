# Repository, website and releases

WeCom Chat CLI follows the WeCom Calendar and Channel repository infrastructure:
GitHub Actions validates changes, Pages serves `docs/`, and a maintainer's version
tag triggers native binaries, checksums and npm OIDC publication.

The source is **0.2.0-dev**. No version tag, GitHub Release or npm package has been
published. Repository and Pages setup are distinct from releasing a binary.

## Repository bootstrap

The intended public repository is `AngelMsger/wecom-chat-cli`; the documentation
URL is `https://angelmsger.github.io/wecom-chat-cli/`. The bootstrap script uses the
active `gh` identity and never reads or prints a token:

```sh
python3 scripts/setup-github.py --dry-run
python3 scripts/setup-github.py --apply
```

Run the apply command from a clean, committed `main` with credentials authorized
to create/manage this repository, push workflows and enable Pages. It creates the
repository if absent, verifies its public identity, sets only this checkout's
origin, pushes main without force, enables Actions-based Pages, configures
metadata/private vulnerability reporting and protects main. It never creates a
tag, publishes npm or modifies a sibling repository. A rerun resumes configuration;
errors are surfaced rather than treated as success.

The required checks are `test (ubuntu-latest)`, `test (macos-latest)` and
`isolation`. Protection applies to administrators; force pushes and deletion are
disabled, and changes after bootstrap use pull requests. No reviewer count is
required, but conversations must be resolved. Release publication has no additional
environment approval gate. Verify the configuration and actual CI/Pages runs
before describing the remote setup as complete.

If GitHub returns `403 Resource not accessible by integration`, the connected
integration lacks the required administration capability. Create the empty public
repository and grant the integration access, or supply suitable credentials using
the environment's protected settings. Never paste credentials into issues or chat.
A repository with only Contents permission still cannot configure Pages/protection.

## Continuous integration and website

`ci.yml` runs formatting, vet, race tests, compiled CLI/socket tests, build,
deterministic documentation checks, static site links, release contract tests,
npm installer tests and vulnerability scanning on Linux and macOS. The Linux
`isolation` job cross-builds four targets and runs the distinct-UID fixture in a
network-disabled pinned container. All fixtures use synthetic conversations.

`pages.yml` checks generated documentation and links, uploads `docs/`, then uses
GitHub's official Pages deployment action on relevant main changes or a manual
workflow dispatch. Set Pages **Source: GitHub Actions**. The site needs no runtime
backend, CDN, external font, npm dependency or secret. Keep Markdown canonical and
run `make docs` after command or narrative changes. The old per-command Markdown
paths remain available alongside the searchable HTML reference.

Dependabot follows Calendar's policy: track released `agent-cli-kit` versions;
review other dependency upgrades explicitly. No local `replace` or workspace
module override may reach a release.

## Release preparation

1. Run the validation below with `GOWORK=off`. Confirm CI and Pages status.
2. Resolve vulnerabilities and review the deployment/acceptance limitations.
3. Set `build/npm/package.json` to the intended semantic version. Update the
   changelog, site/package release status and companion Skill if its content changed.
4. Merge the prepared changes through a pull request. A maintainer can then push
   the intended `vMAJOR.MINOR.PATCH` tag from that clean main commit.

Do not create a tag merely to test the pipeline. A version-tag push authorizes
publication automatically; there is no separate release approval click.
The binaries receive version, commit and build time via `pkg/constants` and ldflags.
`wecom-chat-cli version` reports that metadata as JSON.

## npm setup before the first release

The package is `@angelmsger/wecom-chat-cli`. The registry package and its trusted
publisher must be provisioned before expecting automatic publication to succeed.
This cannot be configured by a GitHub repository workflow alone. Follow
[npm's trusted-publisher guidance](https://docs.npmjs.com/trusted-publishers/).
If the package does not exist, its owner must perform the initial authorized
publication/bootstrap through npm before configuring the package publisher.

| Trusted publisher field | Value |
| --- | --- |
| Organization or user | `AngelMsger` |
| Repository | `wecom-chat-cli` |
| Workflow filename | `release.yml` |
| Environment | Leave blank, matching Calendar |

The workflow grants `id-token: write`, uses Node 24's bundled npm, and supplies no
registry token or `registry-url`. Do not add a long-lived token or a custom
publisher. The repository URL's `AngelMsger` casing must match provenance.
Unlike Channel, Chat has no previously bound `release` environment to preserve.

## Pipeline and artifact contract

A `v*` tag runs checks, cross-compiles, generates SHA-256 checksums, creates or
refreshes the GitHub Release, and publishes the npm wrapper. It copies the license
and matching companion Skill into the package. npm is skipped if the exact version
already exists. The GitHub Release remains available if npm configuration fails.

The stable assets are:

```text
wecom-chat-cli-darwin-amd64
wecom-chat-cli-darwin-arm64
wecom-chat-cli-linux-amd64
wecom-chat-cli-linux-arm64
checksums.txt
```

The installer downloads from
`https://github.com/AngelMsger/wecom-chat-cli/releases/download/v<version>/<asset>`.
Missing, duplicate or mismatched checksums fail installation. Only HTTPS GitHub
asset hosts are accepted, with size/time/redirect limits. Runtime agent commands
never trigger an implicit download. Windows is excluded because this broker
requires Unix sockets and Unix filesystem security.

Reruns refresh GitHub assets and skip existing npm versions, following the family
pipeline. Inspect the source and partial outcome first; never move a published tag
or assume an npm version can be overwritten. The local release tests execute these
shell steps with substitutes for `gh`, npm and the compiler; they never publish.

## Validation

```sh
GOWORK=off make lint test e2e build docs-check site-check release-tests
npm test --prefix build/npm
GOWORK=off make vuln cross
# Disposable network-disabled root setup; see verification.md for the image.
WECOM_CHAT_TEST_ROOT=1 GOFLAGS="-exec=$PWD/scripts/container-exec.sh" make isolation-check
```

The generic [host integration](host-integration.md) still requires a real host
adapter, sandbox, per-session memory and independent reviewer. Publishing code,
packages or a website does not certify a live WeCom deployment.

## Family alignment

| Reference | Reused pattern | Intentional difference |
| --- | --- | --- |
| WeCom Calendar | Public GitHub repo, Pages, kit-only Dependabot, tag → binaries/checksums → npm OIDC | Chat is Unix-only and requires task capabilities |
| WeCom Channel | Linux/macOS CI, Linux isolation, Markdown-to-HTML site, checksum-required installer, release contract tests | Smart-bot WebSocket tasks; no Channel contexts or reaction commands |
| Bitbucket, Confluence, Jenkins, Jira, OpenObserve | Same native build and tag-driven release pipeline | No changes to those repositories |
| Prometheus | Same common release flow | Its extra duplicate-publication error parser is not copied |
| agent-cli-kit | Published pinned module for shared CLI behavior | No kit changes or release requested |
