# Security reports

This project is a prerelease. Only the current main branch receives fixes; no
stable release or live-tenant certification is claimed.

Use the repository's private vulnerability reporting option under **Security →
Report a vulnerability** when enabled. If unavailable, ask a maintainer to enable
a private reporting channel without posting exploit details or sensitive data in
a public issue. Never include real Bot Secrets, task tokens or employee messages.

Provide a synthetic reproduction, affected version/commit, expected boundary and
observed impact. The threat model and required host isolation are documented in
[security and deployment](docs/security.md). A heuristic miss is not proof of an
authorization bypass; describe which broker or host capability became accessible.
