import { useEffect, useState, type FormEvent } from 'react'
import { AlertCircleIcon } from 'lucide-react'
import { api, formatTime, type Project, type Release } from '@/api'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
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

export default function ReleasesPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [projectId, setProjectId] = useState('1')
  const [formProjectId, setFormProjectId] = useState('1')
  const [releases, setReleases] = useState<Release[]>([])
  const [open, setOpen] = useState(false)
  const [version, setVersion] = useState('')
  const [error, setError] = useState('')
  const [formError, setFormError] = useState('')

  useEffect(() => {
    api.projects().then((p) => {
      setProjects(p)
      if (p.length) {
        const first = String(p[0].id)
        if (!p.find((x) => String(x.id) === projectId)) {
          setProjectId(first)
        }
        if (!p.find((x) => String(x.id) === formProjectId)) {
          setFormProjectId(first)
        }
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
    if (!version.trim() || !formProjectId) return
    try {
      await api.createRelease({
        project_id: Number(formProjectId),
        version: version.trim(),
      })
      setVersion('')
      setFormError('')
      setOpen(false)
      if (formProjectId === projectId) {
        setReleases(await api.releases(projectId))
      } else {
        setProjectId(formProjectId)
      }
    } catch (err) {
      setFormError(String(err))
    }
  }

  const projectItems = projects.map((p) => ({
    label: p.name,
    value: String(p.id),
  }))

  return (
    <section className="flex flex-col gap-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <h1 className="font-heading text-2xl font-medium tracking-tight">
          Releases
        </h1>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button />}>Register release</DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Register release</DialogTitle>
              <DialogDescription>
                Link a version to a project for release health.
              </DialogDescription>
            </DialogHeader>
            <form onSubmit={onCreate} className="flex flex-col gap-4">
              <FieldGroup className="grid gap-3">
                <Field>
                  <FieldLabel>Project</FieldLabel>
                  <Select
                    items={projectItems}
                    value={formProjectId}
                    onValueChange={(v) =>
                      setFormProjectId(v == null ? '' : String(v))
                    }
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="Select project" />
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
                <Field>
                  <FieldLabel htmlFor="version">Version</FieldLabel>
                  <Input
                    id="version"
                    value={version}
                    onChange={(e) => setVersion(e.target.value)}
                    placeholder="1.2.3"
                  />
                </Field>
              </FieldGroup>
              {formError && (
                <Alert variant="destructive">
                  <AlertCircleIcon />
                  <AlertTitle>Create failed</AlertTitle>
                  <AlertDescription>{formError}</AlertDescription>
                </Alert>
              )}
              <DialogFooter>
                <Button type="submit" disabled={!formProjectId}>
                  Create
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

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
