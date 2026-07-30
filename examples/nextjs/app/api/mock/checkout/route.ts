import * as Sentry from "@sentry/nextjs"

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export async function POST(request: Request) {
  return Sentry.startSpan(
    { name: "POST /api/mock/checkout", op: "http.server" },
    async () => {
      let body: { userId?: string; amount?: number } = {}
      try {
        body = (await request.json()) as { userId?: string; amount?: number }
      } catch {
        body = {}
      }

      await Sentry.startSpan(
        { name: "validate.cart", op: "function" },
        async () => {
          await sleep(30 + Math.floor(Math.random() * 40))
        }
      )

      await Sentry.startSpan(
        { name: "payment.charge", op: "http.client" },
        async () => {
          await sleep(80 + Math.floor(Math.random() * 60))
          // ~30% chance of payment failure inside the transaction
          if (Math.random() < 0.3) {
            const err = new Error(
              "sample checkout payment declined (sentry-lite)"
            )
            Sentry.captureException(err, {
              tags: { service: "sample", surface: "mock-api", route: "checkout" },
            })
            throw err
          }
        }
      )

      await Sentry.startSpan(
        { name: "INSERT INTO orders", op: "db" },
        async () => {
          await sleep(40 + Math.floor(Math.random() * 40))
        }
      )

      return Response.json({
        ok: true,
        orderId: `ord_${Date.now()}`,
        userId: body.userId ?? "1",
        amount: body.amount ?? 42,
      })
    }
  )
}
