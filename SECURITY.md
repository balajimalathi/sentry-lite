# Security Policy

## Supported versions

sentry-lite is **alpha** software. Security fixes are applied on the `main`
branch only. There are no long-term support releases yet.

| Version | Supported |
|---------|-----------|
| `main` (alpha) | ✅ |
| Pre-release tags | ❌ unless noted otherwise |

## Reporting a vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Please report privately via
[GitHub Security Advisories](https://github.com/balajimalathi/sentry-lite/security/advisories/new)
with:

- A description of the issue and its impact
- Steps to reproduce (PoC if possible)
- Affected version / commit SHA
- Any suggested fix (optional)

You should receive an acknowledgement within a few days. We will work with you
to understand and address the issue, and will credit reporters who wish to be
named once a fix is released (unless you prefer anonymity).

## Scope notes

- Ingest endpoints authenticate via project DSN public keys (by design, like
  Sentry). Treat DSNs as secrets for write access to a project.
- Management UI and `/api/internal/*` require `ADMIN_TOKEN` when configured.
  Never expose that token publicly.
- Production deploys should bind the app to localhost and terminate TLS on a
  reverse proxy (see README Security section).
