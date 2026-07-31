import * as Sentry from "@sentry/nextjs"

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export async function GET() {
  return Sentry.startSpan(
    { name: "GET /api/mock/slow", op: "http.server" },
    async () => {
      await Sentry.startSpan(
        { name: "upstream.slow_api", op: "http.client" },
        async () => {
          await sleep(1500)
        }
      )

      return Response.json({
        ok: true,
        latency_ms: 1500,
        message: "intentionally slow mock response",
      })
    }
  )
}
