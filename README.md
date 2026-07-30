# sentry-lite

[![Status](https://img.shields.io/badge/status-alpha-orange)](https://github.com/balajimalathi/sentry-lite)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)

**Lean, self-hostable error monitoring** with Sentry-compatible ingest. Drop-in for official Sentry SDKs — point the DSN at your instance.

> **Alpha** — APIs, storage schema, and UI may change without a stable upgrade path. Suitable for evaluation and early self-hosting, not production-critical workloads yet. See [Security](#security) before exposing anything to the internet.

## Features

- **Sentry-compatible ingest** — envelopes + legacy store; use `@sentry/*` SDKs as-is
- **Issue triage UI** — filter by environment, release, tags; resolve / ignore / assign
- **Performance** — transactions, spans, p95/p99, trace detail
- **Crons** — heartbeat monitors with missed-check alerts
- **Releases** — register versions and track issue/event health
- **Alerts** — Slack, email, webhook, Telegram
- **Single Go binary** + SQLite + Redpanda (Kafka-compatible) bus
- **Docker image** on GHCR: `ghcr.io/balajimalathi/sentry-lite`

## Architecture

| Piece | Role |
|-------|------|
| Go API + processor | Ingest, processing, management API (one binary) |
| SQLite | Metadata / issues / projects |
| Redpanda | Kafka-compatible ingest bus (separate Compose stack) |
| React (Vite) UI | Triage dashboard (Bun to build / develop) |

```
SDK ──DSN──▶ Go API ──▶ Redpanda ──▶ processor ──▶ SQLite
                │
                └── serves UI (+ /api/internal/*)
```

## Quick start

### Prerequisites

- [Go](https://go.dev/) 1.25+
- [Bun](https://bun.sh)
- [Docker](https://docs.docker.com/get-docker/) (Redpanda)

### Local (recommended)

```bash
cp .env.example .env
# edit ADMIN_TOKEN for gateway login; defaults work locally

docker compose -f docker-compose.redpanda.yml up -d
go run ./cmd/sentry-lite-tui
```

The TUI starts Redpanda (if needed), the Go API, and the Vite UI with live logs.

Or start pieces yourself:

```bash
docker compose -f docker-compose.redpanda.yml up -d
go run ./cmd/sentry-lite          # http://localhost:8080
cd web && bun install && bun run dev   # http://localhost:5173
```

Defaults (bare-metal / TUI):

| | |
|--|--|
| HTTP | `http://localhost:8080` |
| SQLite | `./data/sentry-lite.db` |
| Redpanda | `localhost:19092` (Compose app uses `redpanda:9092`) |

`go run` / the TUI load `.env` automatically (existing shell env wins).

### Docker (prebuilt image)

Merges to `main` publish `ghcr.io/balajimalathi/sentry-lite` (`latest`, `sha-<shortsha>`). Redpanda stays a separate stack.

```bash
cp .env.example .env
# set ADMIN_TOKEN (e.g. openssl rand -hex 32) and optional HTTP_PORT
docker compose -f docker-compose.redpanda.yml up -d
docker compose up -d
```

Or plain Docker (Redpanda must already be up on `sentry-lite-net`):

```bash
docker run --rm \
  --env-file .env \
  -e HTTP_ADDR=:8080 \
  -e SQLITE_PATH=/data/sentry-lite.db \
  -e DATA_DIR=/data \
  -e WEB_DIST=/app/web/dist \
  -e REDPANDA_BROKERS=redpanda:9092 \
  -p 127.0.0.1:8080:8080 \
  -v sentry-lite-data:/data \
  --network sentry-lite-net \
  ghcr.io/balajimalathi/sentry-lite:latest
```

Build from source:

```bash
docker compose -f docker-compose.redpanda.yml up -d
docker compose up --build
```

The app listens on `127.0.0.1:${HTTP_PORT:-8080}` only. Put **host nginx** (or Caddy) in front for TLS — see [Security](#security).

## Seed DSN

On first boot a demo project is created:

```
http://a1b2c3d4e5f6789012345678abcdef01@localhost:8080/1
```

Point any official Sentry SDK at this DSN (change host/path only). Create more projects in the UI (Projects → New project).

### SDK configuration

Environment, release, and tags are **not** set in the triage UI — they arrive on each event from the SDK:

```ts
Sentry.init({
  dsn: '...',
  environment: 'production',
  release: 'my-app@1.2.3',
  tracesSampleRate: 1.0,
})

Sentry.captureException(err, {
  tags: { service: 'api' },
  user: { id: 'user-1' },
})
```

### Telegram alerts

Channel `telegram` stores `botToken|chatId`. Message your bot once (to learn the chat id), then create the rule — sentry-lite sends a sample message and fails create if Telegram rejects it.

## Examples & tools

| Path | Purpose |
|------|---------|
| [`examples/node-sdk`](examples/node-sdk) | Bun smoke test (errors + performance) |
| [`examples/nextjs`](examples/nextjs) | Next.js playground (`@sentry/nextjs`) |
| [`docs/load-test.md`](docs/load-test.md) | Headless 1M-event load test |
| `go run ./cmd/sentry-lite-load` | Interactive load-test TUI |
| `go run ./cmd/sentry-lite-release` | Register a release from CLI |

```bash
# errors
cd examples/node-sdk && bun install && bun run send.ts

# performance
bun run send-perf.ts

# cron check-in (create monitor in UI first)
curl -X POST http://localhost:8080/api/cron/check-in/<token>
```

Release CLI:

```bash
go run ./cmd/sentry-lite-release -version=1.2.3 [-project=1] [-token="$ADMIN_TOKEN"]
# or SENTRY_LITE_TOKEN
```

## API surface

| Endpoint | Purpose |
|----------|---------|
| `POST /api/{project_id}/envelope/` | Sentry envelope ingest (events + transactions) |
| `POST /api/{project_id}/store/` | Legacy JSON store |
| `POST /api/cron/check-in/{token}` | Cron heartbeat check-in |
| `GET /api/internal/projects` | Project list |
| `POST /api/internal/projects` | Create project `{ name, slug? }` → project + DSN |
| `GET /api/internal/facets` | Distinct env/release/tag values |
| `GET /api/internal/issues` | Issue list (filters: `project_id`, `environment`, `release`, `q`, tags, `from`/`to`) |
| `GET /api/internal/issues/{id}` | Issue + latest event |
| `PATCH /api/internal/issues/{id}` | `{ "status": "open\|resolved\|ignored", "assignee": "..." }` |
| `GET /api/internal/transactions?project_id=` | Transaction list with p95/p99 (24h) |
| `GET /api/internal/transaction?project_id=&name=` | Samples + spans |
| `GET /api/internal/traces/{trace_id}` | Trace detail + related error issues |
| `GET /api/internal/crons?project_id=` | Cron monitors |
| `POST /api/internal/crons` | Create monitor |
| `PATCH` / `DELETE /api/internal/crons/{id}` | Update / delete monitor |
| `GET` / `POST /api/internal/releases` | List / register releases |
| `GET` / `POST /api/internal/alerts` | Alert rules (`slack` / `email` / `webhook` / `telegram`) |
| `GET /healthz` | Health check |

## Configuration

| Var | Default |
|-----|---------|
| `HTTP_ADDR` | `:8080` |
| `SQLITE_PATH` | `./data/sentry-lite.db` |
| `DATA_DIR` | `./data` |
| `REDPANDA_BROKERS` | `localhost:19092` (Compose: `redpanda:9092`) |
| `INGEST_TOPIC` | `events.ingest` |
| `WEB_DIST` | `./web/dist` |
| `PUBLIC_URL` | `http://localhost:8080` (issue links in alerts) |
| `ADMIN_TOKEN` | _(empty = management API open; set for any real deploy)_ |
| `HTTP_PORT` | `8080` (Compose host bind only) |
| `SENTRY_LITE_TOKEN` | _(release CLI; same as `ADMIN_TOKEN`)_ |
| `ALERT_SMTP` | _(empty = email alerts off)_ |
| `ALERT_FROM` | `sentry-lite@localhost` |

Browser SDK CORS is per project (`allowed_origins`). Empty list = any Origin. The seeded demo project allows `http://localhost:5173`, `:3000`, and `:8080`.

## Security

Management UI and `/api/internal/*` are protected by `ADMIN_TOKEN` when set. Ingest (DSN public key) and cron check-ins (URL token) stay separate.

- **Local / TUI:** put `ADMIN_TOKEN` in `.env` so the Vite UI shows gateway-token login.
- **Any non-local deploy:** use a long random `ADMIN_TOKEN`. UI stores it in `sessionStorage` (1h idle expiry).

### Host reverse proxy (VPS — not in Docker)

Bind the container to localhost; terminate TLS on the host:

```nginx
server {
    listen 443 ssl http2;
    server_name sentry.example.com;

    # ssl_certificate     /etc/letsencrypt/live/sentry.example.com/fullchain.pem;
    # ssl_certificate_key /etc/letsencrypt/live/sentry.example.com/privkey.pem;

    client_max_body_size 32m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

To report a vulnerability, see [SECURITY.md](SECURITY.md).

## Troubleshooting

### Ghost issues / load-test leftovers

Ingest lands in Redpanda first. If you wipe SQLite (or delete a project) but leave a topic backlog, the processor can rewrite old events — often as orphan `project_id`s with no matching project.

**Fix:** always reset SQLite and Redpanda together. Stop the API / TUI / load tool first, then:

```bash
./scripts/wipe-local.sh        # prompts for confirmation
# or
./scripts/wipe-local.sh --yes
```

That deletes `./data/sentry-lite.db*` + `./data/events`, runs `docker compose -f docker-compose.redpanda.yml down -v`, and brings Redpanda back empty. Restart the app and create a fresh project.

Check backlog:

```bash
docker compose -f docker-compose.redpanda.yml exec redpanda rpk group describe sentry-lite-processor
```

Non-zero `LAG` means messages are still waiting to be processed.

### Consumer stuck after hard kill

If ingest returns 200 but issues never appear after a hard kill, the Kafka consumer group may be stuck mid-rebalance:

```bash
docker compose -f docker-compose.redpanda.yml exec redpanda rpk group delete sentry-lite-processor
```

Then restart the app.

## Contributing

Contributions are welcome — please read [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

Copyright 2026 Balaji Malathi.

Licensed under the [Apache License, Version 2.0](LICENSE). See [NOTICE](NOTICE).
