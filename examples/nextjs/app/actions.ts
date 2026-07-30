"use server"

import * as Sentry from "@sentry/nextjs"

export async function triggerServerError() {
  return Sentry.withServerActionInstrumentation(
    "triggerServerError",
    { recordResponse: true },
    async () => {
      throw new Error("sample server action failure (sentry-lite)")
    }
  )
}

export async function captureServerException() {
  return Sentry.withServerActionInstrumentation(
    "captureServerException",
    { recordResponse: true },
    async () => {
      try {
        throw new Error("sample captured server exception (sentry-lite)")
      } catch (e) {
        Sentry.captureException(e, {
          tags: { service: "sample", surface: "server-action" },
        })
        await Sentry.flush(2000)
        return { ok: true as const }
      }
    }
  )
}
