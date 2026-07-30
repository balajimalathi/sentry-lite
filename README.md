# sentry-lite

Lean, self-hostable error monitoring with Sentry-compatible ingest. V2 adds performance tracing and cron/heartbeat monitoring.

## Stack

- Go API + processor (single binary)
- SQLite metadata store
- Redpanda (Kafka-compatible) ingest bus — separate Compose file
- React (Vite) triage UI, built with Bun

## Quick start (local)

Copy env for local API auth (gateway token login):

```bash
cp .env.example .env
# edit ADMIN_TOKEN if you want; defaults work for local
```

`go run` / the TUI load `.env` automatically (existing shell env wins).

### 1. Start Redpanda (separate stack)

```bash
docker compose -f docker-compose.redpanda.yml up -d
```

Kafka listens only on the Docker network (`redpanda:9092`). No host ports are published.

### 2. Run the API

For local `go run`, either run the app via Compose (recommended) or temporarily publish a loopback Kafka port on Redpanda. With Compose, the app uses `REDPANDA_BROKERS=redpanda:9092`.

```bash
go run ./cmd/sentry-lite
```

Defaults (bare-metal):

- HTTP: `http://localhost:8080`
- SQLite: `./data/sentry-lite.db`
- Redpanda: `localhost:19092` (only if you publish that port for local dev)

To reach internal-only Redpanda from the host for a one-off local run, add a temporary publish, e.g. `"127.0.0.1:19092:9092"` under `ports` in `docker-compose.redpanda.yml`, then set `REDPANDA_BROKERS=localhost:19092`.

### 3. UI (dev)

```bash
cd web
bun install
bun run dev
```

Open `http://localhost:5173` (proxies `/api` to the Go server).

### 4. Production-style (Compose)

Start Redpanda first, then the app. Configure token and host port via `.env`:

```bash
cp .env.example .env
# set ADMIN_TOKEN (e.g. openssl rand -hex 32) and optional HTTP_PORT
docker compose -f docker-compose.redpanda.yml up -d
cd web && bun run build && cd ..
docker compose up --build
```

The app listens on `127.0.0.1:${HTTP_PORT:-8080}` only. Put **host nginx** (on the VPS, not in Docker) in front for TLS and public access — see [Security](#security).

UI + API via nginx (or locally `http://127.0.0.1:8080`).

The app joins the external `sentry-lite-net` network created by the Redpanda stack and connects via `redpanda:9092`. Redpanda is not published to the host.

## Seed DSN

On first boot a demo project is created:

```
http://a1b2c3d4e5f6789012345678abcdef01@localhost:8080/1
```

Point any official Sentry SDK at this DSN (only change the host/path).

You can also create projects in the UI (Projects → New project). Copy the returned DSN into your SDK.

### Environment, release, and tags

These are **not** set in the triage UI — they arrive on each event from the SDK:

```ts
Sentry.init({
  dsn: '...',
  environment: 'production',
  release: 'my-app@1.2.3',
  tracesSampleRate: 1.0, // enable performance transactions
})

Sentry.captureException(err, {
  tags: { service: 'api' },
  user: { id: 'user-1' },
})
```

Issue filters (environment / release / tag dropdowns) read distinct values already stored from ingested events.

### Telegram alerts

Channel `telegram` stores `botToken|chatId`. Message your bot once (so you know the chat id), then create the rule — sentry-lite sends a “connected” sample message and fails create if Telegram rejects it.

## Smoke test (Node SDK + Bun)

Errors:

```bash
cd examples/node-sdk
bun install
bun run send.ts
```

Performance transactions:

```bash
bun run send-perf.ts
```

Then open Issues / Performance in the UI.

### Next.js sample (Bun + shadcn)

```bash
cd examples/nextjs
bun install
bun run dev
```

Points `@sentry/nextjs` at project 3 — full feature playground (errors, mock APIs / performance, releases, crons, alerts). See [`examples/nextjs/README.md`](examples/nextjs/README.md).

### Load test TUI

Stress/peak ingest against the API (errors, transactions, crons, releases — same mix as the playground):

```bash
go run ./cmd/sentry-lite-load
```

Headless 1M-event run: see [`docs/load-test.md`](docs/load-test.md).

### Cron check-in

Create a monitor in the Crons UI, then:

```bash
curl -X POST http://localhost:8080/api/cron/check-in/<token>
```

## Dev TUI (start everything)

One command starts Redpanda (Docker), the Go API, and the Vite web app, with live logs:

```bash
go run ./cmd/sentry-lite-tui
```

On launch it runs:

1. `docker compose -f docker-compose.redpanda.yml up -d` (+ log tail)
2. `bun run dev` in `web/`
3. `go run ./cmd/sentry-lite`

Left sidebar panels: redpanda · api · web · stats (live RAM/CPU/disk cards).

Keys: `↑`/`↓` select service · `j`/`k` scroll · `1–4` / `tab` shortcuts · `a` restart all · `s` restart · `x` stop · `r` refresh stats · `q` quit (stops API + web; leaves Redpanda running). Mouse wheel scrolls the log/stats pane.

## API surface

| Endpoint | Purpose |
|----------|---------|
| `POST /api/{project_id}/envelope/` | Sentry envelope ingest (events + transactions) |
| `POST /api/{project_id}/store/` | Legacy JSON store |
| `POST /api/cron/check-in/{token}` | Cron heartbeat check-in |
| `GET /api/internal/projects` | Project list |
| `POST /api/internal/projects` | Create project `{ name, slug? }` → project + DSN |
| `GET /api/internal/facets` | Distinct env/release/tag values (`project_id` optional) |
| `GET /api/internal/issues` | Issue list (`project_id`, `environment`, `release`, `q`, `tag`/`tag_key`+`tag_value`, `from`, `to`) |
| `GET /api/internal/issues/{id}` | Issue + latest event |
| `PATCH /api/internal/issues/{id}` | `{ "status": "open\|resolved\|ignored", "assignee": "..." }` |
| `GET /api/internal/transactions?project_id=` | Transaction list with p95/p99 (24h) |
| `GET /api/internal/transaction?project_id=&name=` | Transaction samples + spans |
| `GET /api/internal/traces/{trace_id}` | Trace detail + related error issues |
| `GET /api/internal/crons?project_id=` | Cron monitors |
| `POST /api/internal/crons` | Create monitor `{ project_id, name, schedule_sec, grace_sec? }` |
| `PATCH /api/internal/crons/{id}` | Update monitor |
| `DELETE /api/internal/crons/{id}` | Delete monitor |
| `GET /api/internal/releases?project_id=` | Release health (issue/event counts) |
| `POST /api/internal/releases` | Register release `{ project_id, version, ref?, url? }` |
| `GET /api/internal/alerts?project_id=` | Alert rules |
| `POST /api/internal/alerts` | Create rule (`slack` / `email` / `webhook` / `telegram`; triggers include `cron_missed`) |
| `GET /healthz` | Health check |

Release CLI:

```bash
go run ./cmd/sentry-lite-release -version=1.2.3 [-project=1] [-token="$ADMIN_TOKEN"]
```

Or set `SENTRY_LITE_TOKEN`.

## Security

Management UI and `/api/internal/*` are protected by a shared `ADMIN_TOKEN` (gateway token) when that env var is set. Ingest (`DSN` public key) and cron check-ins (URL token) stay separate.

- **Local / TUI:** put `ADMIN_TOKEN` in `.env` (see `.env.example`) so the Vite UI at `:5173` shows the gateway-token login. Leave it unset only if you want an open management API (process logs a warning).
- **Any non-local deploy:** set a long random `ADMIN_TOKEN` in `.env`. Sign in to the UI with that token (stored in browser `sessionStorage`; expires after 1 hour of inactivity). Share the same token so others can sign in from their browsers.

### Host nginx (VPS — not in Docker)

Bind the container to localhost (`127.0.0.1:8080` in Compose). Terminate TLS and proxy on the VPS with nginx (or Caddy). Example:

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

SDKs use the public hostname in the DSN; nginx forwards envelopes to the app, which still validates the project public key. Do not put nginx inside the Compose stack for this setup.

## Env vars

| Var | Default |
|-----|---------|
| `HTTP_ADDR` | `:8080` |
| `SQLITE_PATH` | `./data/sentry-lite.db` |
| `DATA_DIR` | `./data` |
| `REDPANDA_BROKERS` | `localhost:19092` (Compose app uses `redpanda:9092`) |
| `INGEST_TOPIC` | `events.ingest` |
| `WEB_DIST` | `./web/dist` |
| `PUBLIC_URL` | `http://localhost:8080` (issue links in alerts) |
| `ADMIN_TOKEN` | _(empty = management API open; required by Compose / `.env`)_ |
| `HTTP_PORT` | `8080` (Compose host bind only; see `.env.example`) |
| `SENTRY_LITE_TOKEN` | _(release CLI; same value as `ADMIN_TOKEN`)_ |
| `ALERT_SMTP` | _(empty = email alerts disabled)_ |
| `ALERT_FROM` | `sentry-lite@localhost` |

Browser SDK CORS is configured per project (`allowed_origins` on create). An empty list allows any Origin. The seeded demo project allows `http://localhost:5173`, `http://localhost:3000`, and `http://localhost:8080`.

## Troubleshooting

If ingest returns 200 but issues never appear after hard-killing the app, the Kafka consumer group may be stuck mid-rebalance:

```bash
docker compose -f docker-compose.redpanda.yml exec redpanda rpk group delete sentry-lite-processor
```

Then restart the app.
