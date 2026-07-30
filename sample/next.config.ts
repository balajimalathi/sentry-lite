import type { NextConfig } from "next"
import { withSentryConfig } from "@sentry/nextjs"

const nextConfig: NextConfig = {}

export default withSentryConfig(nextConfig, {
  // Local sentry-lite: no SaaS org/project upload or tunnelRoute.
  silent: true,
})
