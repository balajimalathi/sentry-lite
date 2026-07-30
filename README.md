# sentry-lite

Lean, self-hostable error monitoring with Sentry-compatible ingest. V2 adds performance tracing and cron/heartbeat monitoring.

## Stack

- Go API + processor (single binary)
- SQLite metadata store
- Redpanda (Kafka-compatible) ingest bus — separate Compose file
- React (Vite) triage UI, built with Bun

## Quick start (local)

### 1. Start Redpanda (separate stack)

```bash
docker compose -f docker-compose.redpanda.yml up -d
```

### 2. Run the API

```bash
go run ./cmd/sentry-lite
```

Defaults:

- HTTP: `http://localhost:8080`
- SQLite: `./data/sentry-lite.db`
- Redpanda: `localhost:19092`

### 3. UI (dev)

```bash
cd web
bun install
bun run dev
```

Open `http://localhost:5173` (proxies `/api` to the Go server).

### 4. Production-style (Compose)

Start Redpanda first, then the app:

```bash
docker compose -f docker-compose.redpanda.yml up -d
cd web && bun run build && cd ..
docker compose up --build
```

UI + API: `http://localhost:8080`

The app joins the external `sentry-lite-net` network created by the Redpanda stack and connects via `redpanda:9092`.

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
go run ./cmd/sentry-lite-release -version=1.2.3 [-project=1]
```

## Env vars

| Var | Default |
|-----|---------|
| `HTTP_ADDR` | `:8080` |
| `SQLITE_PATH` | `./data/sentry-lite.db` |
| `DATA_DIR` | `./data` |
| `REDPANDA_BROKERS` | `localhost:19092` |
| `INGEST_TOPIC` | `events.ingest` |
| `CORS_ORIGINS` | `http://localhost:5173,http://localhost:3000` |
| `WEB_DIST` | `./web/dist` |
| `PUBLIC_URL` | `http://localhost:8080` (issue links in alerts) |
| `ALERT_SMTP` | _(empty = email alerts disabled)_ |
| `ALERT_FROM` | `sentry-lite@localhost` |

## Troubleshooting

If ingest returns 200 but issues never appear after hard-killing the app, the Kafka consumer group may be stuck mid-rebalance:

```bash
docker compose -f docker-compose.redpanda.yml exec redpanda rpk group delete sentry-lite-processor
```

Then restart the app.
