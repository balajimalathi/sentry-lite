import * as Sentry from "@sentry/nextjs"

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

const USERS: Record<string, { id: string; name: string; email: string }> = {
  "1": { id: "1", name: "Ada Lovelace", email: "ada@example.com" },
  "2": { id: "2", name: "Alan Turing", email: "alan@example.com" },
  "3": { id: "3", name: "Grace Hopper", email: "grace@example.com" },
}

export async function GET(
  _request: Request,
  context: { params: Promise<{ id: string }> }
) {
  const { id } = await context.params

  return Sentry.startSpan(
    { name: `GET /api/mock/users/${id}`, op: "http.server" },
    async () => {
      await Sentry.startSpan(
        { name: "SELECT user BY id", op: "db" },
        async () => {
          await sleep(50 + Math.floor(Math.random() * 50))
        }
      )

      const user = USERS[id]
      if (!user) {
        return Response.json(
          { error: "user not found", id },
          { status: 404 }
        )
      }

      return Response.json({ user })
    }
  )
}
