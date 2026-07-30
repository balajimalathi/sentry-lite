import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

type PageHeaderProps = {
  title: ReactNode
  description?: ReactNode
  /** Primary actions kept on the right (never stack below the title). */
  actions?: ReactNode
  className?: string
}

/**
 * Shared list/main page chrome: title + optional subtitle on the left,
 * primary action(s) on the right.
 *
 * For action buttons, pair an icon with {@link PageHeaderActionLabel} so mobile
 * shows icon-only while desktop keeps the label. Set `aria-label` on the button.
 */
export function PageHeader({
  title,
  description,
  actions,
  className,
}: PageHeaderProps) {
  return (
    <div
      className={cn('flex items-start justify-between gap-3', className)}
    >
      <div className="min-w-0 flex flex-col gap-1">
        <h1 className="font-heading text-2xl font-medium tracking-tight">
          {title}
        </h1>
        {description ? (
          <p className="text-sm text-muted-foreground">{description}</p>
        ) : null}
      </div>
      {actions ? (
        <div className="flex shrink-0 items-center gap-2">{actions}</div>
      ) : null}
    </div>
  )
}

/** Hide action label on small screens; pair with an icon + button `aria-label`. */
export function PageHeaderActionLabel({
  children,
}: {
  children: ReactNode
}) {
  return <span className="hidden sm:inline">{children}</span>
}
