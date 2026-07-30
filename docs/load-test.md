# Load testing sentry-lite

Interactive and headless load harness that simulates production-style traffic against local sentry-lite ingest — same feature mix as the [Next.js playground](../examples/nextjs/components/feature-playground.tsx).

## Prerequisites

1. sentry-lite API running (`go run ./cmd/sentry-lite` or `go run ./cmd/sentry-lite-tui`)
2. Redpanda up (for events to be processed after ingest)
3. A valid project + DSN (seed project `1` or your playground project `3`)

## Interactive TUI

```bash
go run ./cmd/sentry-lite-load
```

Setup screen: configure mode, total, workers, RPS, DSN. Press **Enter** to start.

| Key | Action |
|-----|--------|
| `↑`/`↓` | Select field |
| `space` | Edit field / toggle stress↔peak |
| `Enter` | Start (confirms if total > 100k) |
| `s` | Start / stop |
| `p` | Pause / resume |
| `k` | Crash probe — flood API at max concurrency |
| `q` | Quit |

Live panels show throughput, latency percentiles, per-feature counts, `healthz` status, and `data/` disk usage. A **crash banner** appears when the API was healthy then becomes unreachable.

## Headless (CI / scripted)

```bash
go run ./cmd/sentry-lite-load --headless --yes \
  --mode=stress \
  --total=1000000 \
  --rps=5000 \
  --workers=200 \
  --dsn='http://fa41276d7b8b1b7e58c5aa350c965f04@localhost:8080/3'
```

Peak mode:

```bash
go run ./cmd/sentry-lite-load --headless --yes \
  --mode=peak \
  --total=500000 \
  --peak-rps=10000 --burst-sec=30 --idle-sec=10 --idle-rps=500 \
  --workers=200
```

Runs over **100k events** require `--yes` (disk safety).

## What gets sent

Weighted random mix (defaults mirror the playground):

| Category | Payload |
|----------|---------|
| errors | `exception` events |
| messages | `captureMessage`-style events |
| context | user + breadcrumbs + exception |
| fingerprint | shared `demo-group` fingerprint |
| transactions | mock API shapes (users, slow, checkout) with spans |
| checkout_fail | transaction or linked payment error (~30%) |
| releases | `sample@0.1.0` / `sample@0.2.0` errors |
| crons | check-in ok/error (if monitor can be created) |

Alerts are seeded once via internal API (webhook rules). Cron monitor uses slug `sample-heartbeat`.

Traffic goes to `POST /api/{project}/store/` with `X-Sentry-Auth` — same path as official SDKs.

## What to watch

- **TUI / headless**: sent/ok/5xx/timeout, p50/p95/p99, in-flight
- **Triage UI**: Issues, Performance, Releases, Crons, Alerts
- **Disk**: `./data/events/` grows ~one JSON file per event; 1M events can use significant disk

## Env

| Variable | Purpose |
|----------|---------|
| `SENTRY_DSN` / `NEXT_PUBLIC_SENTRY_DSN` / `LOAD_DSN` | Default DSN |
| `CORS_ORIGINS` | Not needed (direct HTTP, not browser) |

## Flags

```
--dsn           Sentry DSN
--mode          stress | peak
--total         max events (default 1000000)
--workers       goroutines (default 200)
--rps           stress sustained RPS (default 5000)
--peak-rps      peak burst RPS (default 10000)
--burst-sec     peak burst duration
--idle-sec      peak idle duration
--idle-rps      peak baseline RPS
--data-dir      sentry-lite data dir for disk stats (default ./data)
--headless      no TUI
--yes           skip large-run confirmation
```
