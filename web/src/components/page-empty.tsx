import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'

export function PageEmpty({
  title,
  description,
  action,
}: {
  title: string
  description: string
  action?: ReactNode
}) {
  return (
    <Empty className="border border-dashed">
      <EmptyHeader>
        <EmptyTitle>{title}</EmptyTitle>
        <EmptyDescription>{description}</EmptyDescription>
      </EmptyHeader>
      {action ? <EmptyContent>{action}</EmptyContent> : null}
    </Empty>
  )
}

export function SelectProjectEmpty() {
  return (
    <PageEmpty
      title="Select a project"
      description="This view needs a project. Use the project switcher in the header."
    />
  )
}

export function CreateProjectEmpty() {
  return (
    <PageEmpty
      title="No projects yet"
      description="Create a project, copy its DSN, and point your Sentry SDK at this instance."
      action={
        <Link
          to="/projects"
          className="text-sm font-medium underline-offset-4 hover:underline"
        >
          Go to Projects
        </Link>
      }
    />
  )
}
