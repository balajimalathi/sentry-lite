# V1 Feature Validation Results

**Date:** 2026-07-30 (re-validated after full V1 gap close)  
**Build:** full V1 slice (search + regression policy + releases + alerts + UI polish)  
**Against:** [PLAN.md](PLAN.md) V1 acceptance criteria  

## Environment

| Component | Status |
|-----------|--------|
| Redpanda | `docker compose -f docker-compose.redpanda.yml` — healthy; Kafka internal-only (`redpanda:9092` on `sentry-lite-net`) |
| API | `./bin/sentry-lite` on `:8080` (Compose: `REDPANDA_BROKERS=redpanda:9092`; local host publish needed for bare-metal) |
| UI | `bun run dev` on `http://127.0.0.1:5173` |
| Seed DSN | `http://a1b2c3d4e5f6789012345678abcdef01@localhost:8080/1` |
| Smoke | `examples/node-sdk` (`@sentry/node`) |

Helper script: [scripts/v1_validate.py](scripts/v1_validate.py)

---

## Summary

| § | Area | Result |
|---|------|--------|
| 1 | Ingest & compatibility | **PASS** |
| 2 | Grouping & issues | **PASS** |
| 3 | Triage UI | **PASS** |
| 4 | Tags & search | **PASS** (scale unvalidated) |
| 5 | Alerts | **PASS** |
| 6 | Releases & release health | **PASS** |
| 7 | Operations & footprint | **PARTIAL** (idle memory unmeasured) |

**Verdict:** Full V1 product surface is implemented and smoke-checked. Remaining gap vs PLAN is ops idle-memory measurement only.

---

## 1. Ingest & compatibility — PASS

| Check | Result | Evidence |
|-------|--------|----------|
| Official SDK via DSN only | Pass | `bun run send.ts` → events ingested |
| Envelope endpoint | Pass | 200 + event id |
| Core fields stored | Pass | platform, user, tags, environment, release, frames |

---

## 2. Grouping & issues — PASS

| Check | Result | Evidence |
|-------|--------|----------|
| Same exception groups | Pass | smoke issue count increments |
| Different exception separate | Pass | TypeError vs Error |
| Explicit `fingerprint` | Pass | store API fingerprint override |
| Lifecycle open/resolved/ignored | Pass | PATCH status |
| Regression policy | Pass | Different release vs `last_release`, or 24h quiet window after `resolved_at` |

---

## 3. Triage UI — PASS

| Check | Result | Evidence |
|-------|--------|----------|
| Project list | Pass | `/` |
| Issue filters | Pass | project, env, release, tag `key:value`, q, from/to |
| Owner column + assign | Pass | list column + detail form → PATCH assignee |
| Detail stack / tags / user / timeline | Pass | Issue detail |
| Breadcrumbs panel | Pass | Rendered from latest event payload |
| Resolve / Ignore / Reopen | Pass | Buttons |

---

## 4. Tags & search — PASS

| Check | Result | Evidence |
|-------|--------|----------|
| `environment` / `release` | Pass | query params |
| Tag filter | Pass | `?tag=service:demo` / `tag_key`+`tag_value` (covers `user.id`) |
| Text + message search | Pass | `q` matches title, culprit, message, exception_type |
| Timeframe | Pass | `from` / `to` on `last_seen` |
| Scale (tens of thousands) | **Unvalidated** | Load harness exists (`go run ./cmd/sentry-lite-load`); results TBD per run |

---

## 5. Alerts — PASS

| Check | Result | Evidence |
|-------|--------|----------|
| Rules API | Pass | `POST/GET /api/internal/alerts` |
| Triggers | Pass | `new_issue`, `regressed_issue`, `error_volume` in processor |
| Channels | Pass | Slack webhook, SMTP email (`ALERT_SMTP`), HMAC-signed webhook |
| UI | Pass | `/alerts` create + list |

---

## 6. Releases & release health — PASS

| Check | Result | Evidence |
|-------|--------|----------|
| Events linked via `release` | Pass | issue first/last release |
| Create release API/CLI | Pass | `POST /api/internal/releases`; `sentry-lite-release -version=…` |
| Auto-register on ingest | Pass | processor `UpsertRelease` |
| Health list | Pass | issue_count + event_count per version; UI `/releases` |

---

## 7. Operations & footprint — PARTIAL

| Check | Result | Evidence |
|-------|--------|----------|
| Redpanda + volume | Pass | healthy container |
| Split Compose | Pass | redpanda vs app |
| Idle memory ≤ 100 MB | **Unvalidated** | Not measured this pass |

---

## Reproduction commands

```bash
docker compose -f docker-compose.redpanda.yml up -d
go run ./cmd/sentry-lite
cd examples/node-sdk && bun run send.ts
curl -s 'http://localhost:8080/api/internal/issues?project_id=1&tag=service:demo' | jq .
curl -s -X POST http://localhost:8080/api/internal/releases \
  -H 'Content-Type: application/json' -d '{"project_id":1,"version":"1.2.3"}'
go run ./cmd/sentry-lite-release -version=1.2.3
cd web && bun run dev
```

---

# V2 Feature Validation Results

**Date:** 2026-07-30  
**Against:** [PLAN.md](PLAN.md) V2 acceptance criteria  

## Summary

| § | Area | Result |
|---|------|--------|
| 1 | Performance tracing | **Implemented** — SDK envelope transactions, list p95/p99, detail spans, trace↔issue link |
| 2 | Cron/heartbeat | **Implemented** — register, check-in API, late/missed watcher, `cron_missed` alerts |
| 3 | UI integration | **Implemented** — Performance, Traces, Crons nav in same SPA |
| 4 | Operational impact | **Assumed OK** at target scale (SQLite + Redpanda mixed workload) |

## Smoke commands

```bash
# Performance
cd examples/node-sdk && bun run send-perf.ts
curl -s 'http://localhost:8080/api/internal/transactions?project_id=1' | jq .

# Cron
curl -s -X POST http://localhost:8080/api/internal/crons \
  -H 'Content-Type: application/json' \
  -d '{"project_id":1,"name":"demo-job","schedule_sec":60,"grace_sec":10}'
curl -s -X POST http://localhost:8080/api/cron/check-in/<token>
curl -s 'http://localhost:8080/api/internal/crons?project_id=1' | jq .
```

Helper: [scripts/v2_validate.py](scripts/v2_validate.py)
