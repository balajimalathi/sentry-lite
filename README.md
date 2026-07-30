# sentry-lite

Lean, self-hostable error monitoring with Sentry-compatible ingest.

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

## Smoke test (Node SDK + Bun)

```bash
cd examples/node-sdk
bun install
bun run send.ts
```

Then open Issues in the UI — identical errors should group into one issue.

## API surface

| Endpoint | Purpose |
|----------|---------|
| `POST /api/{project_id}/envelope/` | Sentry envelope ingest |
| `POST /api/{project_id}/store/` | Legacy JSON store |
| `GET /api/internal/projects` | Project list |
| `GET /api/internal/issues` | Issue list (`project_id`, `environment`, `release`, `q`) |
| `GET /api/internal/issues/{id}` | Issue + latest event |
| `PATCH /api/internal/issues/{id}` | `{ "status": "open\|resolved\|ignored" }` |
| `GET /healthz` | Health check |

## Env vars

| Var | Default |
|-----|---------|
| `HTTP_ADDR` | `:8080` |
| `SQLITE_PATH` | `./data/sentry-lite.db` |
| `DATA_DIR` | `./data` |
| `REDPANDA_BROKERS` | `localhost:19092` |
| `INGEST_TOPIC` | `events.ingest` |
| `CORS_ORIGINS` | `http://localhost:5173` |
| `WEB_DIST` | `./web/dist` |

## Troubleshooting

If ingest returns 200 but issues never appear after hard-killing the app, the Kafka consumer group may be stuck mid-rebalance:

```bash
docker compose -f docker-compose.redpanda.yml exec redpanda rpk group delete sentry-lite-processor
```

Then restart the app.
