import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ArrowLeftIcon, HouseIcon, RotateCcwIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'

type ErrorStateProps = {
  icon: ReactNode
  title: string
  description: string
  details?: string
  onRetry?: () => void
  onBack?: () => void
  homeTo?: string
  homeLabel?: string
}

export function ErrorState({
  icon,
  title,
  description,
  details,
  onRetry,
  onBack,
  homeTo = '/',
  homeLabel = 'Go home',
}: ErrorStateProps) {
  return (
    <Empty className="min-h-[50vh] border border-dashed">
      <EmptyHeader>
        <EmptyMedia variant="icon">{icon}</EmptyMedia>
        <EmptyTitle className="text-xl">{title}</EmptyTitle>
        <EmptyDescription>{description}</EmptyDescription>
      </EmptyHeader>
      <EmptyContent>
        {details ? (
          <pre className="max-h-32 w-full overflow-auto rounded-lg bg-muted px-3 py-2 text-left font-mono text-xs text-muted-foreground whitespace-pre-wrap">
            {details}
          </pre>
        ) : null}
        <div className="flex flex-wrap items-center justify-center gap-2">
          {onRetry ? (
            <Button onClick={onRetry}>
              <RotateCcwIcon data-icon="inline-start" />
              Try again
            </Button>
          ) : null}
          <Button
            variant={onRetry ? 'outline' : 'default'}
            render={<Link to={homeTo} />}
          >
            <HouseIcon data-icon="inline-start" />
            {homeLabel}
          </Button>
          {onBack ? (
            <Button variant="ghost" onClick={onBack}>
              <ArrowLeftIcon data-icon="inline-start" />
              Go back
            </Button>
          ) : null}
        </div>
      </EmptyContent>
    </Empty>
  )
}
