import * as Sentry from "@sentry/nextjs"

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

const USERS = [
  { id: "1", name: "Ada Lovelace", email: "ada@example.com" },
  { id: "2", name: "Alan Turing", email: "alan@example.com" },
  { id: "3", name: "Grace Hopper", email: "grace@example.com" },
]

export async function GET() {
  return Sentry.startSpan(
    { name: "GET /api/mock/users", op: "http.server" },
    async () => {
      await Sentry.startSpan(
        { name: "cache.get users", op: "cache" },
        async () => {
          await sleep(40 + Math.floor(Math.random() * 40))
        }
      )

      await Sentry.startSpan(
        { name: "SELECT * FROM users", op: "db" },
        async () => {
          await sleep(60 + Math.floor(Math.random() * 80))
        }
      )

      return Response.json({ users: USERS })
    }
  )
}
