---
name: sentry-lite-nextjs
description: Integrates sentry-lite as a drop-in SDK for Sentry in Next.js App Router apps using official @sentry/nextjs. Uses graft CLI to inspect examples/nextjs and the target app, then wires DSN, instrumentation, withSentryConfig, and capture/tracing. Use when adding sentry-lite, pointing a Sentry DSN at a self-hosted instance, replacing Sentry SaaS, or editing instrumentation.ts, sentry.server.config.ts, sentry.edge.config.ts, instrumentation-client.ts, or global-error.tsx.
allowed-tools: Bash(graft *), Bash(npx @nanonets/graft *), Bash(npx -y @nanonets/graft *)
---

# sentry-lite Next.js (drop-in Sentry SDK)

sentry-lite is a drop-in SDK for Sentry: keep `@sentry/nextjs` and point the DSN at the sentry-lite instance. Do not write a custom client.

Canonical wiring lives in `examples/nextjs`. Copy that pattern, not the Sentry SaaS wizard (no org/project upload, no `tunnelRoute`).

## Graft first

Use [graft CLI](https://github.com/NanoNets/Graft) (`graft` or `npx @nanonets/graft`) before and after edits. Prefer graft over reading whole files.

If `graft/` is missing: `graft build` at the repo root (no API key). Then:

```bash
graft check
graft map
graft grep "Sentry.init|withSentryConfig|@sentry/nextjs|NEXT_PUBLIC_SENTRY_DSN"
```

When this sentry-lite repo is the workspace, inspect the canonical example:

```bash
graft ask "How is @sentry/nextjs wired for sentry-lite?" --in examples/nextjs --source
graft grep "Sentry.init|withSentryConfig|captureRequestError|onRouterTransitionStart" --in examples/nextjs
graft skeleton examples/nextjs/instrumentation.ts
graft skeleton examples/nextjs/app/actions.ts
```

`graft check` must stay OK. If it fails, rebuild (`graft build`) before trusting ask/grep/skeleton.

Decide from graft output:

- **No `@sentry/nextjs`** → full install below
- **Already on Sentry SaaS** → keep the SDK files; swap DSN; strip org/project/`tunnelRoute` from `withSentryConfig`
- **Already on sentry-lite** → stop; only change env, CORS, or usage

Do not copy the playground UI, mock APIs, or `/api/internal/*` helpers unless the user asked for the demo. Those are example-only — see [reference.md](reference.md).

## Install

```bash
# bun / npm / pnpm / yarn — match the target app
bun add @sentry/nextjs
```

Pin near `examples/nextjs` (`@sentry/nextjs` ^10, Next.js App Router). Do not add `@sentry/node` / `@sentry/browser` alongside it.

## Files to add

Match `examples/nextjs` names. `instrumentation-client.ts` is the client init (not `sentry.client.config.ts`).

### 1. `instrumentation.ts`

```ts
import * as Sentry from "@sentry/nextjs"

export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    await import("./sentry.server.config")
  }

  if (process.env.NEXT_RUNTIME === "edge") {
    await import("./sentry.edge.config")
  }
}

export const onRequestError = Sentry.captureRequestError
```

### 2. `sentry.server.config.ts` and `sentry.edge.config.ts`

Same `Sentry.init` in both files:

```ts
import * as Sentry from "@sentry/nextjs"

Sentry.init({
  dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
  environment: process.env.NODE_ENV,
  release: process.env.NEXT_PUBLIC_SENTRY_RELEASE,
  tracesSampleRate: 1.0,
  initialScope: {
    tags: { service: "web" },
  },
})
```

`environment`, `release`, and tags are set in the SDK. sentry-lite UI does not configure them.

### 3. `instrumentation-client.ts`

```ts
import * as Sentry from "@sentry/nextjs"

Sentry.init({
  dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
  environment: process.env.NODE_ENV,
  release: process.env.NEXT_PUBLIC_SENTRY_RELEASE,
  tracesSampleRate: 1.0,
  initialScope: {
    tags: { service: "web" },
  },
})

export const onRouterTransitionStart = Sentry.captureRouterTransitionStart
```

### 4. Wrap Next config

Preserve existing `nextConfig`. Only wrap it:

```ts
import type { NextConfig } from "next"
import { withSentryConfig } from "@sentry/nextjs"

const nextConfig: NextConfig = {
  // existing options unchanged
}

export default withSentryConfig(nextConfig, {
  // Local sentry-lite: no SaaS org/project upload or tunnelRoute.
  silent: true,
})
```

Never set `org`, `project`, `authToken`, `tunnelRoute`, or source-map upload for sentry-lite.

### 5. `app/global-error.tsx`

App Router requires a client global error boundary that reports the error:

```tsx
"use client"

import * as Sentry from "@sentry/nextjs"
import NextError from "next/error"
import { useEffect } from "react"

export default function GlobalError({
  error,
}: {
  error: Error & { digest?: string }
}) {
  useEffect(() => {
    Sentry.captureException(error)
  }, [error])

  return (
    <html>
      <body>
        <NextError statusCode={0} />
      </body>
    </html>
  )
}
```

If `global-error.tsx` already exists, add the `captureException` effect — do not replace the user's UI.

## Env

```bash
NEXT_PUBLIC_SENTRY_DSN=http://<public_key>@<host>:<port>/<project_id>
```

Example (local): `http://…@localhost:8080/1`

Create the project in sentry-lite (Projects → New project) and paste that DSN. Seed DSN on first boot is project `1`.

Optional:

| Variable | Purpose |
|---|---|
| `NEXT_PUBLIC_SENTRY_RELEASE` | Release string (e.g. `my-app@1.2.3`) |
| `SENTRY_LITE_URL` | Management API base (`http://localhost:8080`) — playground crons/alerts only |

Do not commit real DSNs. Put them in `.env.local`.

## CORS

Browser envelopes POST cross-origin to sentry-lite. Set **Allowed origins** on the project (one URL per line), e.g. `http://localhost:3000`. Empty list allows any Origin.

## Runtime usage

Use `@sentry/nextjs` as Sentry documents. After a server-side `captureException` / `captureMessage` that must land before the action returns, `await Sentry.flush(2000)`.

```ts
Sentry.captureException(err, { tags: { service: "api" } })
Sentry.captureMessage("something happened", { level: "info" })
Sentry.setUser({ id, email })
Sentry.addBreadcrumb({ category: "auth", message: "login", level: "info" })

await Sentry.startSpan({ name: "GET /api/widgets", op: "http.server" }, async () => {
  await Sentry.startSpan({ name: "SELECT * FROM widgets", op: "db" }, async () => {
    /* … */
  })
})

return Sentry.withServerActionInstrumentation("myAction", { recordResponse: true }, async () => {
  /* … */
})
```

## Verify

```bash
graft check
graft grep "Sentry.init|withSentryConfig|onRequestError|onRouterTransitionStart|NEXT_PUBLIC_SENTRY_DSN"
```

Required hits:

- `sentry.server.config.ts` / `sentry.edge.config.ts` / `instrumentation-client.ts` — `Sentry.init` + `NEXT_PUBLIC_SENTRY_DSN`
- `instrumentation.ts` — `register` + `onRequestError`
- `instrumentation-client.ts` — `onRouterTransitionStart`
- Next config — `withSentryConfig` with `silent: true` only

Smoke: throw or `captureException` once, then confirm the issue in sentry-lite (`:5173` UI or `:8080`).

## Do not

- Invent a sentry-lite JS SDK
- Point DSN at `sentry.io` unless the user asked for SaaS
- Enable Sentry wizard SaaS upload / `tunnelRoute`
- Read `graft/` markdown as source of truth for edits — use `graft ask` / `grep` / `skeleton`, then edit the real files
- Treat `/api/internal/*` as part of the drop-in SDK (crons/alerts management; see [reference.md](reference.md))
