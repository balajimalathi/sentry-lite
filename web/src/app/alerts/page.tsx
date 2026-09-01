import { useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef, ColumnFiltersState } from '@tanstack/react-table'
import { AlertCircleIcon, PencilIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { parseAsArrayOf, parseAsString, useQueryStates } from 'nuqs'
import { api, formatRelativeTime, formatTime, type AlertRule } from '@/api'
import { DataTable } from '@/components/data-table/data-table'
import { DataTableColumnHeader } from '@/components/data-table/data-table-column-header'
import { DataTableSkeleton } from '@/components/data-table/data-table-skeleton'
import { ListDataTableFilters } from '@/components/list-data-table-filters'
import { PageEmpty } from '@/components/page-empty'
import {
  PageHeader,
  PageHeaderActionLabel,
} from '@/components/page-header'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
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
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useDataTable } from '@/hooks/use-data-table'
import { EMPTY_PROJECTS } from '@/hooks/use-project-filter'
import { firstFilterValue } from '@/lib/row-filters'

const BASIC_FILTER_KEYS = [
  'project_id',
  'name',
  'trigger',
  'channel',
  'enabled',
] as const

const TRIGGER_OPTIONS = [
  { label: 'New issue', value: 'new_issue' },
  { label: 'Regressed', value: 'regressed_issue' },
  { label: 'Error volume', value: 'error_volume' },
  { label: 'Cron missed', value: 'cron_missed' },
]

const CHANNEL_OPTIONS = [
  { label: 'Webhook', value: 'webhook' },
  { label: 'Slack', value: 'slack' },
  { label: 'Email', value: 'email' },
  { label: 'Telegram', value: 'telegram' },
]

function maskTelegramTarget(target: string) {
  const i = target.indexOf('|')
  if (i <= 0) return '••••'
  return `${target.slice(0, 6)}…|${target.slice(i + 1)}`
}

export default function AlertsPage() {
  const qc = useQueryClient()
  const [basicFilterValues] = useQueryStates({
    project_id: parseAsArrayOf(parseAsString, ','),
    name: parseAsString,
    trigger: parseAsArrayOf(parseAsString, ','),
    channel: parseAsArrayOf(parseAsString, ','),
    enabled: parseAsArrayOf(parseAsString, ','),
  })

  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<AlertRule | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AlertRule | null>(null)
  const [formProjectId, setFormProjectId] = useState('')
  const [name, setName] = useState('New issue alert')
  const [trigger, setTrigger] = useState('new_issue')
  const [channel, setChannel] = useState('webhook')
  const [target, setTarget] = useState('')
  const [botToken, setBotToken] = useState('')
  const [chatId, setChatId] = useState('')
  const [threshold, setThreshold] = useState('10')
  const [windowSec, setWindowSec] = useState('300')
  const [secret, setSecret] = useState('')
  const [formError, setFormError] = useState('')

  const basicColumnFilters = useMemo<ColumnFiltersState>(() => {
    const filters: ColumnFiltersState = []
    for (const key of BASIC_FILTER_KEYS) {
      const value = basicFilterValues[key]
      if (value == null || value === '') continue
      filters.push({ id: key, value })
    }
    return filters
  }, [basicFilterValues])

  const projectsQuery = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.projects(),
  })

  const projects = projectsQuery.data ?? EMPTY_PROJECTS
  const projectOptions = useMemo(
    () => projects.map((p) => ({ label: p.name, value: String(p.id) })),
    [projects]
  )
  const projectItems = projectOptions

  const selectedProjectId = firstFilterValue(
    basicColumnFilters.find((f) => f.id === 'project_id')?.value
  )

  const listProjectId = selectedProjectId
  const defaultProjectId =
    selectedProjectId || (projects[0] ? String(projects[0].id) : '')
  const formProject =
    formProjectId ||
    defaultProjectId ||
    (projects[0] ? String(projects[0].id) : '')

  const rulesQuery = useQuery({
    queryKey: ['alerts', listProjectId || 'all'],
    queryFn: () => api.alerts(listProjectId || undefined),
  })

  function resetForm() {
    setTarget('')
    setBotToken('')
    setChatId('')
    setSecret('')
    setWindowSec('300')
    setThreshold('10')
    setName('New issue alert')
    setTrigger('new_issue')
    setChannel('webhook')
    setFormError('')
    setEditing(null)
  }

  function fillEdit(rule: AlertRule) {
    setEditing(rule)
    setFormProjectId(String(rule.project_id))
    setName(rule.name)
    setTrigger(rule.trigger)
    setChannel(rule.channel)
    setThreshold(String(rule.threshold))
    setWindowSec(String(rule.window_sec || 300))
    setSecret('')
    if (rule.channel === 'telegram') {
      const i = rule.target.indexOf('|')
      setBotToken(i > 0 ? rule.target.slice(0, i) : '')
      setChatId(i > 0 ? rule.target.slice(i + 1) : rule.target)
      setTarget('')
    } else {
      setTarget(rule.target)
      setBotToken('')
      setChatId('')
    }
    setFormError('')
    setOpen(true)
  }

  const createMutation = useMutation({
    mutationFn: () => {
      const resolvedTarget =
        channel === 'telegram' ? `${botToken.trim()}|${chatId.trim()}` : target
      return api.createAlert({
        project_id: Number(formProject),
        name,
        trigger,
        channel,
        target: resolvedTarget,
        threshold: Number(threshold) || 0,
        window_sec: Number(windowSec) || 300,
        secret: channel === 'webhook' ? secret : '',
      })
    },
    onSuccess: () => {
      resetForm()
      setOpen(false)
      void qc.invalidateQueries({ queryKey: ['alerts'] })
    },
    onError: (e) => setFormError(String(e)),
  })

  const updateMutation = useMutation({
    mutationFn: () => {
      if (!editing) throw new Error('No rule selected')
      const resolvedTarget =
        channel === 'telegram' ? `${botToken.trim()}|${chatId.trim()}` : target
      const body: Record<string, unknown> = {
        name,
        trigger,
        channel,
        target: resolvedTarget,
        threshold: Number(threshold) || 0,
        window_sec: Number(windowSec) || 300,
      }
      if (channel === 'webhook' && secret.trim()) {
        body.secret = secret.trim()
      }
      return api.updateAlert(editing.id, body)
    },
    onSuccess: () => {
      resetForm()
      setOpen(false)
      void qc.invalidateQueries({ queryKey: ['alerts'] })
    },
    onError: (e) => setFormError(String(e)),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteAlert(id),
    onSuccess: () => {
      setDeleteTarget(null)
      void qc.invalidateQueries({ queryKey: ['alerts'] })
    },
  })

  const enabledOptions = useMemo(
    () => [
      { label: 'Enabled', value: 'true' },
      { label: 'Disabled', value: 'false' },
    ],
    []
  )

  const columns = useMemo<ColumnDef<AlertRule>[]>(
    () => [
      {
        id: 'name',
        accessorKey: 'name',
        enableColumnFilter: true,
        meta: {
          label: 'Name',
          placeholder: 'Search rules...',
          variant: 'text',
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Name" />
        ),
      },
      {
        id: 'trigger',
        accessorKey: 'trigger',
        enableColumnFilter: true,
        meta: {
          label: 'Trigger',
          variant: 'select',
          options: TRIGGER_OPTIONS,
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Trigger" />
        ),
      },
      {
        id: 'channel',
        accessorKey: 'channel',
        enableColumnFilter: true,
        meta: {
          label: 'Channel',
          variant: 'select',
          options: CHANNEL_OPTIONS,
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Channel" />
        ),
      },
      {
        id: 'target',
        accessorKey: 'target',
        meta: { label: 'Target' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Target" />
        ),
        cell: ({ row }) => (
          <span className="max-w-xs truncate font-mono text-xs">
            {row.original.channel === 'telegram'
              ? maskTelegramTarget(row.original.target)
              : row.original.target}
          </span>
        ),
      },
      {
        id: 'project_id',
        accessorKey: 'project_id',
        enableColumnFilter: true,
        enableHiding: true,
        meta: {
          label: 'Project',
          variant: 'select',
          options: projectOptions,
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Project" />
        ),
        cell: ({ row }) => {
          const project = projects.find((p) => p.id === row.original.project_id)
          return (
            <span className="text-muted-foreground">
              {project?.name ?? row.original.project_id}
            </span>
          )
        },
        filterFn: (row, _id, value) => {
          const selected = Array.isArray(value)
            ? value.map(String)
            : [String(value)]
          return selected.includes(String(row.original.project_id))
        },
      },
      {
        id: 'enabled',
        accessorFn: (r) => String(r.enabled),
        enableColumnFilter: true,
        meta: {
          label: 'Enabled',
          variant: 'select',
          options: enabledOptions,
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Enabled" />
        ),
        cell: ({ row }) => <AlertEnabledSwitch rule={row.original} />,
        filterFn: (row, _id, value) => {
          const selected = Array.isArray(value)
            ? value.map(String)
            : [String(value)]
          return selected.includes(String(row.original.enabled))
        },
      },
      {
        id: 'last_delivered_at',
        accessorFn: (r) => r.last_delivered_at ?? '',
        meta: { label: 'Last delivery' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Last delivery" />
        ),
        cell: ({ row }) => (
          <span
            className="text-muted-foreground"
            title={formatTime(row.original.last_delivered_at)}
          >
            {row.original.last_delivered_at
              ? `${formatRelativeTime(row.original.last_delivered_at)}${
                  row.original.last_delivery_status
                    ? ` · ${row.original.last_delivery_status}`
                    : ''
                }`
              : 'Never'}
          </span>
        ),
      },
      {
        id: 'actions',
        enableSorting: false,
        enableHiding: false,
        header: () => <span className="sr-only">Actions</span>,
        cell: ({ row }) => (
          <div className="flex items-center gap-1">
            <Button
              type="button"
              size="sm"
              variant="outline"
              aria-label={`Edit ${row.original.name}`}
              onClick={() => fillEdit(row.original)}
            >
              <PencilIcon data-icon="inline-start" />
              Edit
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              aria-label={`Delete ${row.original.name}`}
              onClick={() => setDeleteTarget(row.original)}
            >
              <Trash2Icon data-icon="inline-start" />
              Delete
            </Button>
          </div>
        ),
      },
    ],
    [projectOptions, enabledOptions, projects]
  )

  const rawRules = rulesQuery.data ?? []
  const { table } = useDataTable({
    data: rawRules,
    columns,
    pageCount: -1,
    enableAdvancedFilter: false,
    manualFiltering: false,
    manualPagination: false,
    manualSorting: false,
    initialState: {
      sorting: [{ id: 'name', desc: false }],
      pagination: { pageIndex: 0, pageSize: 20 },
    },
  })

  const error = rulesQuery.error
    ? String(rulesQuery.error)
    : deleteMutation.error
      ? String(deleteMutation.error)
      : ''

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!formProject) return
    if (editing) updateMutation.mutate()
    else createMutation.mutate()
  }

  const saving = createMutation.isPending || updateMutation.isPending

  return (
    <section className="flex flex-col gap-4">
      <PageHeader
        title="Alerts"
        description="Notify on issues, volume, and missed crons."
        actions={
          <Dialog
            open={open}
            onOpenChange={(next) => {
              setOpen(next)
              if (!next) resetForm()
              if (next && !editing && !formProjectId && defaultProjectId) {
                setFormProjectId(defaultProjectId)
              }
            }}
          >
            <DialogTrigger
              render={
                <Button
                  disabled={!formProject}
                  aria-label="Create rule"
                />
              }
            >
              <PlusIcon data-icon="inline-start" />
              <PageHeaderActionLabel>Create rule</PageHeaderActionLabel>
            </DialogTrigger>
            <DialogContent className="sm:max-w-lg">
              <DialogHeader>
                <DialogTitle>
                  {editing ? 'Edit alert rule' : 'Create alert rule'}
                </DialogTitle>
                <DialogDescription>
                  Delivered via Slack, email, webhook, or Telegram for the
                  selected project.
                </DialogDescription>
              </DialogHeader>
              <form onSubmit={onSubmit} className="flex flex-col gap-4">
                <FieldGroup className="grid gap-3 sm:grid-cols-2">
                  <Field className="sm:col-span-2">
                    <FieldLabel>Project</FieldLabel>
                    <Select
                      items={projectItems}
                      value={formProject || undefined}
                      disabled={Boolean(editing)}
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
                    <FieldLabel>Name</FieldLabel>
                    <Input
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                    />
                  </Field>
                  <Field>
                    <FieldLabel>Trigger</FieldLabel>
                    <Select
                      items={TRIGGER_OPTIONS}
                      value={trigger}
                      onValueChange={(v) =>
                        setTrigger(String(v ?? 'new_issue'))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          {TRIGGER_OPTIONS.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field>
                    <FieldLabel>Channel</FieldLabel>
                    <Select
                      items={CHANNEL_OPTIONS}
                      value={channel}
                      onValueChange={(v) =>
                        setChannel(String(v ?? 'webhook'))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          {CHANNEL_OPTIONS.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
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
                  <Field>
                    <FieldLabel>Window (seconds)</FieldLabel>
                    <Input
                      value={windowSec}
                      onChange={(e) => setWindowSec(e.target.value)}
                      placeholder="300"
                    />
                    <FieldDescription>
                      Cooldown between deliveries for this rule.
                    </FieldDescription>
                  </Field>
                  {channel === 'webhook' ? (
                    <Field className="sm:col-span-2">
                      <FieldLabel>HMAC secret</FieldLabel>
                      <Input
                        type="password"
                        autoComplete="off"
                        value={secret}
                        onChange={(e) => setSecret(e.target.value)}
                        placeholder={
                          editing
                            ? 'Leave blank to keep the current secret'
                            : 'Signing secret for X-Sentry-Lite-Signature'
                        }
                      />
                      <FieldDescription>
                        Used to sign webhook payloads. Set your own secret —
                        never reuse a shared default.
                      </FieldDescription>
                    </Field>
                  ) : null}
                </FieldGroup>
                {formError && (
                  <Alert variant="destructive">
                    <AlertCircleIcon />
                    <AlertTitle>
                      {editing ? 'Save failed' : 'Create failed'}
                    </AlertTitle>
                    <AlertDescription>{formError}</AlertDescription>
                  </Alert>
                )}
                <DialogFooter>
                  <Button
                    type="submit"
                    disabled={saving || !formProject}
                  >
                    {editing ? 'Save' : 'Create'}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        }
      />

      <AlertDialog
        open={deleteTarget != null}
        onOpenChange={(next) => {
          if (!next) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogMedia>
              <Trash2Icon />
            </AlertDialogMedia>
            <AlertDialogTitle>Delete alert rule?</AlertDialogTitle>
            <AlertDialogDescription>
              This permanently deletes{' '}
              <span className="font-medium text-foreground">
                {deleteTarget?.name}
              </span>{' '}
              and its delivery history. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deleteMutation.isPending || deleteTarget == null}
              onClick={() => {
                if (deleteTarget) deleteMutation.mutate(deleteTarget.id)
              }}
            >
              Delete rule
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {error && (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {rulesQuery.isLoading || projectsQuery.isLoading ? (
        <DataTableSkeleton columnCount={7} filterCount={4} rowCount={6} />
      ) : rawRules.length === 0 ? (
        <PageEmpty
          title="No alert rules"
          description="Create a rule to notify on new issues, volume, or missed crons."
        />
      ) : (
        <DataTable table={table}>
          <ListDataTableFilters table={table} />
        </DataTable>
      )}
    </section>
  )
}

function AlertEnabledSwitch({ rule }: { rule: AlertRule }) {
  const qc = useQueryClient()
  const mutation = useMutation({
    mutationFn: (enabled: boolean) => api.updateAlert(rule.id, { enabled }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['alerts'] })
    },
  })

  return (
    <Switch
      size="sm"
      checked={rule.enabled}
      disabled={mutation.isPending}
      aria-label={rule.enabled ? `Disable ${rule.name}` : `Enable ${rule.name}`}
      onCheckedChange={(checked) => mutation.mutate(Boolean(checked))}
    />
  )
}
