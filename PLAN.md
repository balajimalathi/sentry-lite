# Sentry-lite Plan and Acceptance Criteria

## Overview

Sentry-lite is a lean, self-hostable error and performance monitoring service built with Go, SQLite, local disk storage, and Redpanda as the event bus. It aims to offer Sentry-compatible ingest endpoints and a lightweight UI so users can switch seamlessly from Sentry for core workflows while avoiding the full observability bloat.[^1]

The plan is structured into three delivery bands:
- **V1 (Must-have)**: Core error monitoring, issue grouping, triage UI, tags/search, alerts, releases, and basic release health.
- **V2 (Nice-to-have)**: Simple performance tracing for backend endpoints and cron/heartbeat monitoring.
- **Advanced (Maybe never)**: Session replays, logs/metrics dashboards, AI assistant, and a deep workflow engine.

Each section below includes scope, technical notes, and explicit acceptance criteria.

***

## V1: Core Error Monitoring and Triage

### Scope

V1 focuses on replicating the most widely used Sentry capabilities: real-time error ingestion, grouping into issues, a triage user interface, basic search/filtering, releases, and alerting.[^2][^3]

#### Functional scope

- Sentry-compatible ingest endpoints for error events (JSON/envelope format), per-project DSNs.
- Error normalization: exception type, message, stack trace, culprit, request metadata, user context, environment, release, tags.
- Issue grouping: fingerprinting based on exception type, normalized stack frame, and message pattern.
- Issue lifecycle: open, resolved, ignored/muted, regression detection.
- Triage UI: project list, issue list, issue detail page with event stream and stack trace.
- Tags/search: filter by project, environment, release, tag key/value, timeframe, text query.
- Alerts: rules on new issues, regressed issues, error rate spikes; delivery via Slack, email, and webhooks.[^1][^2]
- Releases: ability to register releases, show per-release issue counts, and mark introduction release for an issue.[^2]

#### Technical scope

- Single Go web/API service exposing:
  - Sentry-compatible `store` endpoints (errors, events, envelopes).
  - Internal JSON API for UI.
- Redpanda as the event bus for ingestion → processing → indexing pipeline.
- SQLite as primary metadata store (projects, keys, issues, events index, tags, releases); events may store raw JSON blobs plus normalized columns.[^4][^5]
- Local disk for payloads: stacktrace raw, attachments, source maps.
- Lightweight UI (SPA or server-rendered) styled to match the Sentry-lite brand, centered on basic triage flows.

### Acceptance criteria

#### 1. Ingest & compatibility

- **Sentry SDKs can send error events to Sentry-lite by only changing the DSN base URL, keeping the protocol unchanged.** At least one official Sentry SDK (e.g., Node.js, Python) is successfully configured to send events without custom code changes besides DSN.[^2]
- **Sentry envelope endpoint is supported for error events**, with core fields correctly parsed: `event_id`, `timestamp`, `logger`, `platform`, `exception`, `threads`, `request`, `user`, `tags`, `environment`, `release`.[^2]

#### 2. Grouping & issues

- **Events with the same exception type and top culprit frame are grouped into a single issue by default**, with override fingerprint support via tags or explicit `fingerprint` field.[^2]
- **Issue detail page shows at least**: latest event timestamp, total event count, affected environments, affected releases, and example stack trace.
- **Issue lifecycle states are implemented**: open, resolved, ignored; regressions are detected when a resolved issue receives a new event in a newer release or after a quiet window.[^1][^2]

#### 3. Triage UI

- **Project list view shows recent issue activity per project** (issues count, latest activity timestamp) and basic sorting.[^2]
- **Issue list view supports filtering by project, environment, release, and timeframe**, and displays at minimum: issue title, count, last seen, first seen, environment, and assigned owner.[^2]
- **Issue detail view includes**:
  - Stack trace (collapsed/expanded) with file, function, line numbers.
  - Tags list with key/value pairs.
  - User info (id/email if provided).
  - Breadcrumbs or events timeline (even if basic at V1).

#### 4. Tags & search

- **Tag filtering works on at least these keys**: `environment`, `release`, `user.id`, and one custom tag (e.g., `service`).[^2]
- **Text search across issues matches issue title and exception message**, and is responsive on tens of thousands of events stored in SQLite.[^4]
- **Search results are consistent with filter combinations**; filter + search behaves as intersection, not union.

#### 5. Alerts

- **Alert rules can be created per project** for: new issue, regressed issue, error volume above threshold per time window.[^2]
- **Slack alerts**: a test rule can send to Slack via webhook, including project, issue link, error summary.[^2]
- **Email alerts**: basic text email alerts are sent via SMTP or a pluggable email provider.
- **Webhook alerts**: for at least one issue type (new issue), with signed payloads.

#### 6. Releases & basic release health

- **Releases can be created via API or CLI**, and linked to events via the `release` field.[^2]
- **Issues display "first release" and "last release" fields** based on events.[^2]
- **Basic release health view per project**: a list of releases with total issues and event volume per release.

#### 7. Operations & footprint

- **Single-node deployment using Docker Compose** with one app container, one Redpanda container, and persistent volumes for SQLite and Redpanda data.[^5]
- **Idle memory footprint** of the app container stays within the defined budget (e.g., ≤ 100 MB at rest for the Go service, excluding Redpanda), validated on a reference environment.

***

## V2: Performance Tracing and Cron Monitoring

### Scope

V2 extends Sentry-lite beyond pure error monitoring by introducing simple backend performance tracing and cron/heartbeat monitoring while still avoiding full APM or custom metrics bloat.[^1][^2]

#### Functional scope

- Simple performance tracing for backend endpoints:
  - Transaction events (name, duration, status, sampled spans).
  - Per-transaction latency stats (p95/p99) over recent windows.
  - Linking performance traces to error issues via shared IDs where possible.[^2]
- Cron/heartbeat monitoring:
  - Ability to register cron jobs or heartbeat endpoints.
  - Detect missed runs or delayed runs.
  - Alerts for jobs that fail to report within configured windows.[^2]

#### Technical scope

- Extend Sentry-compatible ingest endpoints to accept transaction/trace event types, following Sentry’s event model for performance where feasible.[^2]
- Event schema additions in SQLite for transaction events: transaction name, duration, status, environment, release, trace ID.
- Background workers reading Redpanda streams to compute rollups for latency statistics (p95/p99) per transaction.
- Cron monitoring modeled as synthetic events or separate tables with last check-in timestamps.

### Acceptance criteria

#### 1. Performance tracing

- **At least one backend SDK (e.g., Go or Node.js) can send transaction events to Sentry-lite using the same Sentry protocol**, with only DSN changes.[^2]
- **Transaction list view per project**: displays transaction name, p95 duration, p99 duration, and request count over a recent time window (e.g., last 24 hours).
- **Transaction detail view**: shows example traces with spans list (even if simplified) and links to related error issues via trace IDs or transaction name.[^2]
- **Latency statistics calculation is correct and performant** on realistic test data sets (thousands of events), with background rollups not blocking ingest.[^5]

#### 2. Cron/heartbeat monitoring

- **Cron jobs can be registered** with a name, expected frequency, grace period, and optional environment.[^2]
- **Heartbeat endpoints**: a simple `POST /cron/check-in/{id}` or similar API records a successful run.[^2]
- **Missed or late runs trigger alerts** using the same Slack/email/webhook channels as error alerts.[^2]
- **Cron status UI**: list of cron jobs with last check-in, next expected run, and status (ok/late/missed).

#### 3. UI integration

- **Performance and cron views are integrated into the same lightweight UI**, accessible via project-level navigation, without introducing heavy dashboard/BI tooling.[^6]
- **Navigation remains frictionless** for error → transaction → cron context switch, i.e., no cross-app redirects.

#### 4. Operational impact

- **Redpanda and Go workers can handle mixed workloads (errors + transactions + cron events)** under the defined scale target without degrading V1 capabilities.[^5]
- **SQLite remains viable for these additional tables** at the targeted scale, or a migration path to Postgres is documented with compatible schema.

***

## Advanced: Replays, Logs/Metrics Dashboards, AI Assistant, Workflow Engine

### Scope

The advanced band lists capabilities that Sentry offers as part of its modern observability platform but that are intentionally deferred or possibly never implemented in Sentry-lite due to complexity, footprint, and product focus.[^1][^2]

#### Functional scope

- Session replays (user session recordings correlated with issues).
- Logs and metrics ingestion with dashboards and queries.
- AI assistant for issue explanation, likely root cause suggestions, and remediation hints.
- Rich workflow engine: auto-assign rules, complex integrations, advanced alert routing.

#### Technical scope

If implemented, these would introduce:
- High-volume data capture and storage for replays and logs, likely requiring object storage and a more powerful analytics engine than SQLite.
- A vector store or at least LLM integration for AI assistant functionality.
- Complex rule configuration UI and rule evaluation engine for workflows.

### Acceptance criteria (only if implemented)

Because these features may never be built, acceptance criteria are framed in terms of minimum viable implementations rather than guarantees.

#### 1. Replays

- **Frontend SDK can record user sessions and upload them to Sentry-lite**, with correlation to error events via shared identifiers.[^2]
- **Issue detail page can display replay timeline or link to replay view** for issues associated with at least one recording.[^2]
- **Replay storage mechanism is scalable beyond local disk**, using object storage and retention policies.

#### 2. Logs & metrics dashboards

- **Sentry-compatible log ingest or OpenTelemetry log pipelines feed structured logs into Sentry-lite**, with filter/search capabilities.[^1]
- **Basic metrics dashboards**: charts for error rate, throughput, latency by endpoint; no complex BI.
- **Underlying storage is capable of efficient aggregate queries** over logs/metrics (likely ClickHouse or equivalent), with documented migration from SQLite for those workloads.[^5]

#### 3. AI assistant

- **Issue detail view includes an AI-generated summary and probable cause section**, based on stack trace, context, and historical similar issues.[^1]
- **Assistant can suggest at least one plausible remediation step** for common error types (e.g., null pointer, DB connectivity, timeout) and is clearly labeled as AI-generated.

#### 4. Workflow engine

- **Users can define rules based on project, environment, file path, team, and tags** to auto-assign issues, change states, or send alerts to specific channels.[^2]
- **Rule evaluation is reliable and performant** as event volume grows.

#### 5. Non-goals

- These advanced features **must not compromise the primary goal of being a lean, self-hostable, portable Sentry-lite**, and any implementation must preserve the ability to run a minimal footprint deployment without them.

***

## Seamless Switching and UI Constraints

### Sentry-compatible endpoints

To enable seamless switching from Sentry:

- **The ingest API must be protocol-compatible with Sentry’s error and transaction endpoints**, so existing Sentry SDK configurations only need DSN base URL changes to talk to Sentry-lite.[^2]
- **Project keys/DSNs are modeled similarly** to Sentry’s concept of projects and DSNs, including public key, secret, and project ID where needed.[^2]

### Lightweight UI matching the project

UI constraints:

- **UI is optimized for triage speed and clarity**, not for marketing or complex BI dashboards.[^6]
- **Pages focus on a few core views**: project overview, issues list, issue detail, performance, cron, settings.
- **Visual design is minimal and modern**, avoiding heavy libraries and animations that inflate bundle size.

Acceptance criteria:

- A **developer familiar with Sentry can navigate Sentry-lite’s UI and understand how to triage issues within minutes**, without reading extensive documentation.[^7]
- **Page load and interaction remain snappy** on modest hardware, aligning with the portable/self-hostable goal.

***

## Naming and Positioning

The project is named **"sentry-lite"**, deliberately conveying that it is:
- Compatible with major Sentry workflows (ingest endpoints, issues, releases, alerts).[^2]
- Lighter, simpler, and more portable than full Sentry or other full-stack observability platforms.[^8]

Positioning criteria:

- Documentation clearly states which Sentry features are supported in V1/V2 and which are non-goals.
- Migration guide shows how to point existing Sentry SDKs to Sentry-lite with minimal changes.
- Users can run Sentry-lite locally or self-host with Docker Compose, and understand resource expectations.

---

## References

1. [It starts with errors. It ends with full-stack application observability.](https://sentry.io/landing/platform/) - Sentry helps developers monitor and fix crashes in real time. Get the details you need to resolve th...

2. [Product Walkthroughs](https://docs.sentry.io/product/)

3. [Sentry Review: Features, Pricing & Alternatives [2025] - DevDepth](https://devdepth.dev/tools/monitoring/sentry) - Complete Sentry review for developers. Features, pricing, pros & cons, and better alternatives. Real...

4. [Self-hosted Data Flow](https://develop.sentry.dev/self-hosted/data-flow/)

5. [Snuba Architecture Overview](https://getsentry.github.io/snuba/architecture/overview.html)

6. [Sentry Review: Pros, Cons, Features, and Pricing](https://thecxlead.com/tools/sentry-review/) - Discover Sentry's pros and cons, pricing, use cases, and how it compares to other digital experience...

7. [Sentry in Six Minutes](https://www.youtube.com/watch?v=4djseRVSan8) - Wondering what Sentry can do to keep you updated on actionable ways to improve your application? Che...

8. [Sentry](https://dev.to/selfhostingsh/sentry-2417) - Why Replace Sentry? Sentry's free tier covers 5,000 errors and 10,000 performance events...

