# sample — Next.js + sentry-lite

Bun + Next.js (App Router) + shadcn/ui demo that sends errors to local sentry-lite with `@sentry/nextjs`.

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
cd sample
bun install
bun run dev
```

Open [http://localhost:3000](http://localhost:3000), click the error buttons, then check **Issues** in the triage UI ([http://localhost:5173](http://localhost:5173) or [http://localhost:8080](http://localhost:8080)).

## CORS

Browser events POST cross-origin to `:8080`. sentry-lite defaults include `http://localhost:3000`. If you use another origin, set:

```bash
CORS_ORIGINS=http://localhost:5173,http://localhost:3000
```

## Env

Copy `.env.example` → `.env.local` if needed. Override with `NEXT_PUBLIC_SENTRY_DSN`.
