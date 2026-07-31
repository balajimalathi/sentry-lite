import * as Sentry from "@sentry/nextjs"

Sentry.init({
  dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
  environment: "development",
  release: "sample@0.1.0",
  tracesSampleRate: 1.0,
  initialScope: {
    tags: { service: "sample" },
  },
})

export const onRouterTransitionStart = Sentry.captureRouterTransitionStart
