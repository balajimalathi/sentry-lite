import { useEffect, useState, type FormEvent } from 'react'
import { api, type AlertRule, type Project } from '@/api'
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

export default function AlertsPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [projectId, setProjectId] = useState('1')
  const [rules, setRules] = useState<AlertRule[]>([])
  const [name, setName] = useState('New issue Slack')
  const [trigger, setTrigger] = useState('new_issue')
  const [channel, setChannel] = useState('webhook')
  const [target, setTarget] = useState('')
  const [threshold, setThreshold] = useState('10')
  const [error, setError] = useState('')

  useEffect(() => {
    api.projects().then((p) => {
      setProjects(p)
      if (p[0]) setProjectId(String(p[0].id))
    })
  }, [])

  useEffect(() => {
    api
      .alerts(projectId)
      .then(setRules)
      .catch((e) => setError(String(e)))
  }, [projectId])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    try {
      await api.createAlert({
        project_id: Number(projectId),
        name,
        trigger,
        channel,
        target,
        threshold: Number(threshold) || 0,
        window_sec: 300,
        secret: channel === 'webhook' ? 'dev-secret' : '',
      })
      setTarget('')
      setRules(await api.alerts(projectId))
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
      <h1 className="font-heading text-2xl font-medium tracking-tight">Alerts</h1>
      <p className="text-sm text-muted-foreground">
        Rules for new issues, regressions, and error volume. Channels: Slack
        webhook URL, email address (needs ALERT_SMTP), or signed webhook.
      </p>
      {error && (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <form onSubmit={onCreate} className="flex flex-col gap-3">
        <FieldGroup className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <Field>
            <FieldLabel>Project</FieldLabel>
            <Select
              items={projectItems}
              value={projectId}
              onValueChange={(v) => setProjectId(v == null ? '1' : String(v))}
            >
              <SelectTrigger>
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
          <Field>
            <FieldLabel>Name</FieldLabel>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </Field>
          <Field>
            <FieldLabel>Trigger</FieldLabel>
            <Select
              items={[
                { label: 'New issue', value: 'new_issue' },
                { label: 'Regressed', value: 'regressed_issue' },
                { label: 'Error volume', value: 'error_volume' },
              ]}
              value={trigger}
              onValueChange={(v) => setTrigger(String(v ?? 'new_issue'))}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value="new_issue">New issue</SelectItem>
                  <SelectItem value="regressed_issue">Regressed</SelectItem>
                  <SelectItem value="error_volume">Error volume</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <Field>
            <FieldLabel>Channel</FieldLabel>
            <Select
              items={[
                { label: 'Webhook', value: 'webhook' },
                { label: 'Slack', value: 'slack' },
                { label: 'Email', value: 'email' },
              ]}
              value={channel}
              onValueChange={(v) => setChannel(String(v ?? 'webhook'))}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value="webhook">Webhook</SelectItem>
                  <SelectItem value="slack">Slack</SelectItem>
                  <SelectItem value="email">Email</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <Field>
            <FieldLabel>Target (URL / email)</FieldLabel>
            <Input
              value={target}
              onChange={(e) => setTarget(e.target.value)}
              placeholder="https://hooks.slack.com/... or you@example.com"
            />
          </Field>
          <Field>
            <FieldLabel>Volume threshold</FieldLabel>
            <Input
              value={threshold}
              onChange={(e) => setThreshold(e.target.value)}
              placeholder="10"
            />
          </Field>
        </FieldGroup>
        <Button type="submit" className="w-fit">
          Create rule
        </Button>
      </form>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Trigger</TableHead>
            <TableHead>Channel</TableHead>
            <TableHead>Target</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rules.map((r) => (
            <TableRow key={r.id}>
              <TableCell>{r.name}</TableCell>
              <TableCell>{r.trigger}</TableCell>
              <TableCell>{r.channel}</TableCell>
              <TableCell className="max-w-xs truncate font-mono text-xs">
                {r.target}
              </TableCell>
            </TableRow>
          ))}
          {rules.length === 0 && (
            <TableRow>
              <TableCell colSpan={4} className="text-muted-foreground">
                No alert rules yet.
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </section>
  )
}
