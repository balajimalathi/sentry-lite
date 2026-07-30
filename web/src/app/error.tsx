import { Link, isRouteErrorResponse, useNavigate, useRouteError } from 'react-router-dom'
import { AlertCircleIcon, FileQuestionIcon } from 'lucide-react'
import { ErrorState } from '@/components/error-state'
import { Separator } from '@/components/ui/separator'

function errorMessage(error: unknown): {
  title: string
  description: string
  details?: string
  notFound: boolean
} {
  if (isRouteErrorResponse(error)) {
    if (error.status === 404) {
      return {
        title: 'Page not found',
        description: "This URL doesn't match any page in sentry-lite.",
        notFound: true,
      }
    }
    return {
      title: 'Something went wrong',
      description:
        error.statusText ||
        'An unexpected error occurred while loading this page.',
      details: typeof error.data === 'string' ? error.data : undefined,
      notFound: false,
    }
  }

  if (error instanceof Error) {
    return {
      title: 'Something went wrong',
      description: 'An unexpected error occurred while loading this page.',
      details: error.message,
      notFound: false,
    }
  }

  return {
    title: 'Something went wrong',
    description: 'An unexpected error occurred while loading this page.',
    notFound: false,
  }
}

export default function RouteErrorPage({
  standalone = false,
}: {
  standalone?: boolean
}) {
  const error = useRouteError()
  const navigate = useNavigate()
  const { title, description, details, notFound } = errorMessage(error)

  const content = (
    <ErrorState
      icon={notFound ? <FileQuestionIcon /> : <AlertCircleIcon />}
      title={title}
      description={description}
      details={details}
      onRetry={notFound ? undefined : () => window.location.reload()}
      onBack={() => navigate(-1)}
      homeTo="/"
      homeLabel="Go home"
    />
  )

  if (!standalone) return content

  return (
    <div className="flex min-h-svh w-full flex-col gap-6 px-6 py-5 pb-12">
      <header className="flex flex-col gap-4">
        <Link
          to="/"
          className="font-mono text-lg font-bold tracking-tight text-foreground"
        >
          sentry-lite
        </Link>
        <Separator />
      </header>
      <main className="flex flex-1 flex-col gap-4">{content}</main>
    </div>
  )
}

export function StandaloneErrorPage() {
  return <RouteErrorPage standalone />
}
