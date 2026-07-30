import * as Sentry from "@sentry/nextjs"

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export async function GET() {
  return Sentry.startSpan(
    { name: "GET /api/mock/boom", op: "http.server" },
    async () => {
      await Sentry.startSpan(
        { name: "prepare.boom", op: "function" },
        async () => {
          await sleep(40)
        }
      )

      throw new Error("sample mock API boom (sentry-lite)")
    }
  )
}
