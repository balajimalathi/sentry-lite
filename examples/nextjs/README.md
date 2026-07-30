# sentry-lite-nextjs-example — Next.js + sentry-lite

Bun + Next.js (App Router) + shadcn/ui playground that exercises **all** sentry-lite features via `@sentry/nextjs`.

DSN (project 3):

```
http://fa41276d7b8b1b7e58c5aa350c965f04@localhost:8080/3
```

## Run

Terminal 1 — start sentry-lite (API + UI + Redpanda):

```bash
go run ./cmd/sentry-lite-tui
```

Or API only: `go run ./cmd/sentry-lite`.

Terminal 2:

```bash
cd examples/nextjs
bun install
bun run dev
```

Open [http://localhost:3000](http://localhost:3000), then use the sectioned playground. Triage UI: [http://localhost:5173](http://localhost:5173) or [http://localhost:8080](http://localhost:8080).

## What each section does

| Section | Actions | sentry-lite UI |
|---------|---------|----------------|
| **Errors** | Client/server throw + `captureException` | Issues |
| **Context & grouping** | `captureMessage`, user + breadcrumbs, shared `fingerprint` | Issues (detail / filters) |
| **Performance / mock APIs** | `fetch` → `/api/mock/*` with nested `startSpan` (users, checkout, slow, boom) | Performance, Traces |
| **Releases** | Exceptions with `sample@0.1.0` and `sample@0.2.0` | Releases |
| **Crons** | Ensure monitor + check-in ok/error | Crons |
| **Alerts** | Seed webhook rules (`new_issue`, `error_volume`) | Alerts |

### Mock API routes

| Route | Behavior |
|-------|----------|
| `GET /api/mock/users` | Nested cache + db spans, JSON list |
| `GET /api/mock/users/[id]` | Lookup; `404` if missing |
| `POST /api/mock/checkout` | validate → charge → persist; ~30% payment error (linked to transaction) |
| `GET /api/mock/slow` | ~1.5s latency |
| `GET /api/mock/boom` | Always throws |

Tracing is enabled (`tracesSampleRate: 1.0`) on client, server, and edge.

### Crons

Click **Ensure monitor** to create/reuse `sample-heartbeat` on project 3 (schedule 60s, grace 30s), or set `CRON_CHECKIN_TOKEN` in `.env.local`. Then use **Check-in OK** / **Check-in error**.

Skip check-ins past schedule + grace to see `late` / `missed` in the Crons UI.

### Alerts

**Create demo alert rules** POSTs webhook rules to sentry-lite’s internal API. Delivery to Slack / email / Telegram needs real credentials configured in the Alerts UI — the seed only populates rules so you can fire `new_issue` / `error_volume` by sending errors from the playground.

## CORS

Browser events POST cross-origin to `:8080`. sentry-lite defaults include `http://localhost:3000`. If you use another origin, set:

```bash
CORS_ORIGINS=http://localhost:5173,http://localhost:3000
```

## Env

Copy `.env.example` → `.env.local` if needed:

| Variable | Purpose |
|----------|---------|
| `NEXT_PUBLIC_SENTRY_DSN` | Ingest DSN (project 3 by default) |
| `SENTRY_LITE_URL` | Base URL for cron/alert setup (`http://localhost:8080`) |
| `CRON_CHECKIN_TOKEN` | Optional fixed monitor token |
