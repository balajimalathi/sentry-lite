# examples/nextjs playground (not required for drop-in)

The Next.js example is a feature playground against a local sentry-lite instance. Do **not** copy this into a product app unless the user asked to exercise Issues / Performance / Crons / Alerts.

Canonical package: `examples/nextjs` (`@sentry/nextjs`, Next.js App Router). Inspect with:

```bash
graft ask "What does the Next.js playground send to sentry-lite?" --in examples/nextjs --source
graft grep "startSpan|withServerActionInstrumentation|flush|SENTRY_LITE_URL" --in examples/nextjs
graft skeleton examples/nextjs/app/actions.ts
```

## Layout

| Path | Role |
|---|---|
| `instrumentation.ts` | Load server/edge init; `onRequestError` |
| `instrumentation-client.ts` | Browser `Sentry.init`; `onRouterTransitionStart` |
| `sentry.server.config.ts` / `sentry.edge.config.ts` | Node / Edge `Sentry.init` |
| `next.config.ts` | `withSentryConfig(config, { silent: true })` |
| `app/global-error.tsx` | App Router `captureException` |
| `app/actions.ts` | Server actions + optional management API |
| `app/api/mock/*` | Nested `startSpan` demo routes |
| `components/feature-playground.tsx` | UI to fire each feature |
| `.env.example` | `NEXT_PUBLIC_SENTRY_DSN`, `SENTRY_LITE_URL`, `CRON_CHECKIN_TOKEN` |

Demo init in the example hard-codes `environment: "development"`, `release: "sample@0.1.0"`, `tags.service: "sample"`. Product apps should use real env/release (see SKILL.md).

## Mock APIs / performance

Each Route Handler wraps work in `Sentry.startSpan`. Nested `op` values used: `http.server`, `http.client`, `db`, `cache`, `function`.

| Route | Behavior |
|---|---|
| `GET /api/mock/users` | cache + db spans, JSON list |
| `GET /api/mock/users/[id]` | db span; 404 if missing |
| `POST /api/mock/checkout` | validate → charge → persist; ~30% `captureException` + throw |
| `GET /api/mock/slow` | ~1.5s `http.client` span |
| `GET /api/mock/boom` | always throws |

Tracing is on (`tracesSampleRate: 1.0`) on client, server, and edge.

## Server actions

All demo actions use `Sentry.withServerActionInstrumentation(name, { recordResponse: true }, fn)`. Captures call `await Sentry.flush(2000)` before returning.

Fingerprint demo: two `captureMessage` calls share `fingerprint: ["demo-group"]`.

Release demo: `Sentry.withScope` + `addEventProcessor` to override `event.release` to `sample@0.1.0` then `sample@0.2.0`.

## Management API (sentry-lite only)

Not Sentry protocol. Base URL: `SENTRY_LITE_URL` (default `http://localhost:8080`). Example hard-codes `PROJECT_ID = 3`.

If `ADMIN_TOKEN` is set on the server, these internal routes need that token. Ingest (DSN) does not.

**Crons**

- List: `GET /api/internal/crons?project_id=`
- Create: `POST /api/internal/crons` `{ project_id, name, slug, schedule_sec, grace_sec, environment }`
- Check-in: `POST /api/cron/check-in/{token}` `{ status: "ok" | "error", duration_ms }`
- Example reuses slug `sample-heartbeat` or `CRON_CHECKIN_TOKEN`

**Alerts**

- `POST /api/internal/alerts` webhook rules (`new_issue`, `error_volume`)
- Delivery to Slack / email / Telegram still needs credentials in the Alerts UI

## Run the example

Terminal 1: sentry-lite (`go run ./cmd/sentry-lite-tui` or `go run ./cmd/sentry-lite`).

Terminal 2:

```bash
cd examples/nextjs
bun install
bun run dev
```

Playground: `http://localhost:3000`. Triage: `http://localhost:5173` or `:8080`.
