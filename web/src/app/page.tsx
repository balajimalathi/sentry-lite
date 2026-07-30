import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { AlertCircleIcon, FolderIcon } from 'lucide-react'
import { api, formatTime, type Project } from '@/api'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

export default function HomePage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api
      .projects()
      .then(setProjects)
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false))
  }, [])

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertCircleIcon />
        <AlertTitle>Failed to load projects</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  return (
    <section className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h1 className="font-heading text-2xl font-medium tracking-tight">
          Projects
        </h1>
        <p className="text-sm text-muted-foreground">
          Recent issue activity per project.
        </p>
      </div>

      {!loading && projects.length === 0 ? (
        <Empty className="border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <FolderIcon />
            </EmptyMedia>
            <EmptyTitle>No projects yet</EmptyTitle>
            <EmptyDescription>
              Projects appear when events are ingested.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="overflow-hidden rounded-xl ring-1 ring-foreground/10">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Issues</TableHead>
                <TableHead>Latest activity</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {projects.map((p) => (
                <TableRow key={p.id}>
                  <TableCell>
                    <Link
                      to={`/issues?project_id=${p.id}`}
                      className="font-medium text-primary underline-offset-4 hover:underline"
                    >
                      {p.name}
                    </Link>
                  </TableCell>
                  <TableCell className="font-mono text-muted-foreground">
                    {p.slug}
                  </TableCell>
                  <TableCell>{p.issue_count}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatTime(p.latest_activity_at)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </section>
  )
}
