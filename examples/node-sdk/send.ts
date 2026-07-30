import * as Sentry from '@sentry/node'

Sentry.init({
  dsn:
    process.env.SENTRY_DSN ??
    'http://a1b2c3d4e5f6789012345678abcdef01@localhost:8080/1',
  environment: 'development',
  release: 'demo@1.0.0',
  tracesSampleRate: 0,
})

function boom() {
  throw new Error('sentry-lite smoke test failure')
}

async function main() {
  for (let i = 0; i < 2; i++) {
    try {
      boom()
    } catch (e) {
      Sentry.captureException(e, {
        tags: { service: 'demo' },
        user: { id: 'user-1', email: 'demo@example.com' },
      })
    }
  }
  try {
    throw new TypeError('different top frame')
  } catch (e) {
    Sentry.captureException(e, { tags: { service: 'demo' } })
  }

  await Sentry.flush(5000)
  console.log('sent events to', process.env.SENTRY_DSN ?? 'seed DSN')
}

main()
