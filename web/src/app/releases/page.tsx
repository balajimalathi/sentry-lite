import { useEffect, useState, type FormEvent } from 'react'
import { api, formatTime, type Project, type Release } from '@/api'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { AlertCircleIcon } from 'lucide-react'

export default function ReleasesPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [projectId, setProjectId] = useState('1')
  const [releases, setReleases] = useState<Release[]>([])
  const [version, setVersion] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    api.projects().then((p) => {
      setProjects(p)
      if (p.length && !p.find((x) => String(x.id) === projectId)) {
        setProjectId(String(p[0].id))
      }
    })
  }, [])

  useEffect(() => {
    if (!projectId) return
    api
      .releases(projectId)
      .then(setReleases)
      .catch((e) => setError(String(e)))
  }, [projectId])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    if (!version.trim()) return
    try {
      await api.createRelease({
        project_id: Number(projectId),
        version: version.trim(),
      })
      setVersion('')
      setReleases(await api.releases(projectId))
    } catch (err) {
      setError(String(err))
    }
  }

  const projectItems = projects.map((p) => ({
    label: p.name,
    value: String(p.id),
  }))

  return (
    <section className="flex flex-col gap-4">
      <h1 className="font-heading text-2xl font-medium tracking-tight">
        Releases
      </h1>
      {error && (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <FieldGroup className="grid gap-3 sm:grid-cols-[1fr_auto]">
        <Field>
          <FieldLabel>Project</FieldLabel>
          <Select
            items={projectItems}
            value={projectId}
            onValueChange={(v) => setProjectId(v == null ? '1' : String(v))}
          >
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {projectItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
      </FieldGroup>

      <form onSubmit={onCreate}>
        <FieldGroup className="grid gap-3 sm:grid-cols-[1fr_auto]">
          <Field>
            <FieldLabel htmlFor="version">Register release</FieldLabel>
            <Input
              id="version"
              value={version}
              onChange={(e) => setVersion(e.target.value)}
              placeholder="1.2.3"
            />
          </Field>
          <Field className="justify-end">
            <FieldLabel className="sr-only">Create</FieldLabel>
            <Button type="submit">Create</Button>
          </Field>
        </FieldGroup>
      </form>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Version</TableHead>
            <TableHead>Issues</TableHead>
            <TableHead>Events</TableHead>
            <TableHead>Created</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {releases.map((r) => (
            <TableRow key={r.id}>
              <TableCell className="font-mono">{r.version}</TableCell>
              <TableCell>{r.issue_count}</TableCell>
              <TableCell>{r.event_count}</TableCell>
              <TableCell className="text-muted-foreground">
                {formatTime(r.created_at)}
              </TableCell>
            </TableRow>
          ))}
          {releases.length === 0 && (
            <TableRow>
              <TableCell colSpan={4} className="text-muted-foreground">
                No releases yet. Create one or send events with a release field.
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </section>
  )
}
