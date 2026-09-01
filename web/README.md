# sentry-lite web UI

Vite + React dashboard for the Go API. Bun is the package manager.

```bash
bun install
bun run dev      # http://localhost:5173 (proxies /api to :8080)
bun run build    # output in dist/, served by the Go binary
bun run lint
```

The app is a triage cockpit: projects, issues, performance, traces, crons, releases, and alerts. Auth uses `ADMIN_TOKEN` via the login page.
