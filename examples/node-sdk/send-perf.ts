import * as Sentry from '@sentry/node'

Sentry.init({
  dsn:
    process.env.SENTRY_DSN ??
    'http://a1b2c3d4e5f6789012345678abcdef01@localhost:8080/1',
  environment: 'development',
  release: 'demo@1.0.0',
  tracesSampleRate: 1.0,
})

async function main() {
  await Sentry.startSpan(
    { name: 'GET /demo/perf', op: 'http.server' },
    async () => {
      await Sentry.startSpan({ name: 'db.query', op: 'db' }, async () => {
        await new Promise((r) => setTimeout(r, 25))
      })
      await Sentry.startSpan({ name: 'cache.get', op: 'cache' }, async () => {
        await new Promise((r) => setTimeout(r, 10))
      })
    }
  )

  // A second sample with different latency
  await Sentry.startSpan(
    { name: 'GET /demo/perf', op: 'http.server' },
    async () => {
      await new Promise((r) => setTimeout(r, 80))
    }
  )

  await Sentry.flush(5000)
  console.log('sent transactions to', process.env.SENTRY_DSN ?? 'seed DSN')
}

main()
