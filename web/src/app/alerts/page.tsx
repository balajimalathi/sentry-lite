import { useEffect, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertCircleIcon } from 'lucide-react'
import { api } from '@/api'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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

export default function AlertsPage() {
  const qc = useQueryClient()
  const [projectId, setProjectId] = useState('')
  const [name, setName] = useState('New issue alert')
  const [trigger, setTrigger] = useState('new_issue')
  const [channel, setChannel] = useState('webhook')
  const [target, setTarget] = useState('')
  const [botToken, setBotToken] = useState('')
  const [chatId, setChatId] = useState('')
  const [threshold, setThreshold] = useState('10')
  const [error, setError] = useState('')

  const projectsQuery = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.projects(),
  })

  useEffect(() => {
    const projects = projectsQuery.data
    if (!projects?.length) return
    if (!projectId || !projects.some((p) => String(p.id) === projectId)) {
      setProjectId(String(projects[0].id))
    }
  }, [projectsQuery.data, projectId])

  const rulesQuery = useQuery({
    queryKey: ['alerts', projectId],
    queryFn: () => api.alerts(projectId),
    enabled: !!projectId,
  })

  const createMutation = useMutation({
    mutationFn: () => {
      const resolvedTarget =
        channel === 'telegram' ? `${botToken.trim()}|${chatId.trim()}` : target
      return api.createAlert({
        project_id: Number(projectId),
        name,
        trigger,
        channel,
        target: resolvedTarget,
        threshold: Number(threshold) || 0,
        window_sec: 300,
        secret: channel === 'webhook' ? 'dev-secret' : '',
      })
    },
    onSuccess: () => {
      setTarget('')
      setBotToken('')
      setChatId('')
      setError('')
      void qc.invalidateQueries({ queryKey: ['alerts', projectId] })
    },
    onError: (e) => setError(String(e)),
  })

  function onCreate(e: FormEvent) {
    e.preventDefault()
    createMutation.mutate()
  }

  const projects = projectsQuery.data ?? []
  const rules = rulesQuery.data ?? []
  const projectItems = projects.map((p) => ({
    label: p.name,
    value: String(p.id),
  }))

  return (
    <section className="flex flex-col gap-4">
      <h1 className="font-heading text-2xl font-medium tracking-tight">Alerts</h1>
      <p className="text-sm text-muted-foreground">
        Rules for new issues, regressions, and error volume. Channels: Slack
        webhook URL, email (ALERT_SMTP), signed webhook, or Telegram (bot token
        + chat id — sends a connect sample on create).
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
              value={projectId || undefined}
              onValueChange={(v) => setProjectId(v == null ? '' : String(v))}
            >
              <SelectTrigger>
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
                { label: 'Telegram', value: 'telegram' },
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
                  <SelectItem value="telegram">Telegram</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          {channel === 'telegram' ? (
            <>
              <Field>
                <FieldLabel>Bot token</FieldLabel>
                <Input
                  value={botToken}
                  onChange={(e) => setBotToken(e.target.value)}
                  placeholder="123456:ABC-DEF..."
                />
              </Field>
              <Field>
                <FieldLabel>Chat ID</FieldLabel>
                <Input
                  value={chatId}
                  onChange={(e) => setChatId(e.target.value)}
                  placeholder="123456789"
                />
              </Field>
            </>
          ) : (
            <Field>
              <FieldLabel>Target (URL / email)</FieldLabel>
              <Input
                value={target}
                onChange={(e) => setTarget(e.target.value)}
                placeholder="https://hooks.slack.com/... or you@example.com"
              />
            </Field>
          )}
          <Field>
            <FieldLabel>Volume threshold</FieldLabel>
            <Input
              value={threshold}
              onChange={(e) => setThreshold(e.target.value)}
              placeholder="10"
            />
          </Field>
        </FieldGroup>
        <Button
          type="submit"
          className="w-fit"
          disabled={createMutation.isPending || !projectId}
        >
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
                {r.channel === 'telegram'
                  ? maskTelegramTarget(r.target)
                  : r.target}
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

function maskTelegramTarget(target: string) {
  const i = target.indexOf('|')
  if (i <= 0) return '••••'
  return `${target.slice(0, 6)}…|${target.slice(i + 1)}`
}
