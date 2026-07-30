# Contributing to sentry-lite

Thanks for your interest in contributing. This project is in **alpha** — APIs, schemas, and UX may change without notice. Contributions that improve stability, docs, and Sentry SDK compatibility are especially welcome.

## Code of conduct

By participating, you agree to follow our [Code of Conduct](CODE_OF_CONDUCT.md).

## How to contribute

1. **Issues** — Search existing issues before opening a new one. Include repro steps, expected vs actual behavior, and versions (Go, Bun, OS, Docker).
2. **Pull requests** — Fork the repo, create a branch from `main`, keep changes focused, and open a PR with a clear description of *why*.
3. **Security** — Do not file public issues for vulnerabilities. See [SECURITY.md](SECURITY.md).

## Development setup

Prerequisites:

- Go 1.25+
- [Bun](https://bun.sh)
- Docker (for Redpanda)

```bash
cp .env.example .env
docker compose -f docker-compose.redpanda.yml up -d
go run ./cmd/sentry-lite-tui   # or start API + web separately (see README)
```

Smoke tests:

```bash
cd examples/node-sdk && bun install && bun run send.ts
```

## Guidelines

- Match existing Go and TypeScript style in the repo.
- Prefer small, reviewable PRs over large sweeps.
- Add or update tests when changing store/API behavior.
- Do not commit secrets, `.env`, or local data under `data/`.
- Update docs when you change env vars, endpoints, or deploy steps.

## License

Contributions are accepted under the [Apache License 2.0](LICENSE). By submitting a PR, you agree that your contribution is licensed under the same terms.
