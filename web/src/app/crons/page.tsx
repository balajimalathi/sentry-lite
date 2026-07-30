import { useEffect, useState, type FormEvent } from 'react'
import { AlertCircleIcon } from 'lucide-react'
import { api, formatTime, type CronMonitor, type Project } from '@/api'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
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

function statusVariant(status: string) {
  switch (status) {
    case 'ok':
      return 'default' as const
    case 'late':
      return 'secondary' as const
    case 'missed':
      return 'destructive' as const
    default:
      return 'outline' as const
  }
}

export default function CronsPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [projectId, setProjectId] = useState('1')
  const [monitors, setMonitors] = useState<CronMonitor[]>([])
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [scheduleSec, setScheduleSec] = useState('60')
  const [graceSec, setGraceSec] = useState('30')
  const [error, setError] = useState('')
  const [formError, setFormError] = useState('')

  async function load() {
    setMonitors(await api.crons(projectId))
  }

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
    load().catch((e) => setError(String(e)))
  }, [projectId])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    try {
      await api.createCron({
        project_id: Number(projectId),
        name: name.trim(),
        schedule_sec: Number(scheduleSec) || 60,
        grace_sec: Number(graceSec) || 30,
      })
      setName('')
      setFormError('')
      setError('')
      setOpen(false)
      await load()
    } catch (err) {
      setFormError(String(err))
    }
  }

  async function onDelete(id: number) {
    try {
      await api.deleteCron(id)
      await load()
    } catch (err) {
      setError(String(err))
    }
  }

  const projectItems = projects.map((p) => ({
    label: p.name,
    value: String(p.id),
  }))

  const publicBase =
    typeof window !== 'undefined'
      ? window.location.origin.replace(':5173', ':8080')
      : ''

  return (
    <section className="flex flex-col gap-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 className="font-heading text-2xl font-medium tracking-tight">
            Crons
          </h1>
          <p className="text-sm text-muted-foreground">
            Register heartbeat monitors. POST to the check-in URL within the
            schedule window or the monitor goes late/missed.
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button />}>Create monitor</DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Create monitor</DialogTitle>
              <DialogDescription>
                Expected frequency and grace period for project check-ins.
              </DialogDescription>
            </DialogHeader>
            <form onSubmit={onCreate} className="flex flex-col gap-4">
              <FieldGroup className="grid gap-3">
                <Field>
                  <FieldLabel htmlFor="cron-name">Name</FieldLabel>
                  <Input
                    id="cron-name"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="nightly-backup"
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="schedule">Expected every (sec)</FieldLabel>
                  <Input
                    id="schedule"
                    value={scheduleSec}
                    onChange={(e) => setScheduleSec(e.target.value)}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="grace">Grace (sec)</FieldLabel>
                  <Input
                    id="grace"
                    value={graceSec}
                    onChange={(e) => setGraceSec(e.target.value)}
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
                <Button type="submit">Create</Button>
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
            <TableHead>Name</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Last check-in</TableHead>
            <TableHead>Next expected</TableHead>
            <TableHead>Schedule</TableHead>
            <TableHead>Check-in URL</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {monitors.map((m) => (
            <TableRow key={m.id}>
              <TableCell>
                <div className="font-medium">{m.name}</div>
                <div className="font-mono text-xs text-muted-foreground">
                  {m.slug}
                </div>
              </TableCell>
              <TableCell>
                <Badge variant={statusVariant(m.status)}>{m.status}</Badge>
              </TableCell>
              <TableCell className="text-muted-foreground">
                {formatTime(m.last_checkin_at)}
              </TableCell>
              <TableCell className="text-muted-foreground">
                {formatTime(m.next_expected_at)}
              </TableCell>
              <TableCell className="text-sm">
                every {m.schedule_sec}s (+{m.grace_sec}s)
              </TableCell>
              <TableCell className="max-w-xs truncate font-mono text-xs">
                {`POST ${publicBase}/api/cron/check-in/${m.token}`}
              </TableCell>
              <TableCell>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => onDelete(m.id)}
                >
                  Delete
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {monitors.length === 0 && (
            <TableRow>
              <TableCell colSpan={7} className="text-muted-foreground">
                No cron monitors yet.
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </section>
  )
}
