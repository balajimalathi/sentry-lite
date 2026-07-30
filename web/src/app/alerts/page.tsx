import { useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef, ColumnFiltersState } from '@tanstack/react-table'
import { AlertCircleIcon, PlusIcon } from 'lucide-react'
import { parseAsArrayOf, parseAsString, useQueryStates } from 'nuqs'
import { api, type AlertRule } from '@/api'
import { DataTable } from '@/components/data-table/data-table'
import { DataTableColumnHeader } from '@/components/data-table/data-table-column-header'
import { ListDataTableFilters } from '@/components/list-data-table-filters'
import {
  PageHeader,
  PageHeaderActionLabel,
} from '@/components/page-header'
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
import { Skeleton } from '@/components/ui/skeleton'
import { useDataTable } from '@/hooks/use-data-table'
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
  const [name, setName] = useState('New issue alert')
  const [trigger, setTrigger] = useState('new_issue')
  const [channel, setChannel] = useState('webhook')
  const [target, setTarget] = useState('')
  const [botToken, setBotToken] = useState('')
  const [chatId, setChatId] = useState('')
  const [threshold, setThreshold] = useState('10')
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

  const projects = projectsQuery.data ?? []
  const projectOptions = useMemo(
    () => projects.map((p) => ({ label: p.name, value: String(p.id) })),
    [projects]
  )

  const selectedProjectId = firstFilterValue(
    basicColumnFilters.find((f) => f.id === 'project_id')?.value
  )

  // Alerts: empty project = all; for create dialog default to first project.
  const listProjectId = selectedProjectId
  const createProjectId =
    selectedProjectId || (projects[0] ? String(projects[0].id) : '')

  const rulesQuery = useQuery({
    queryKey: ['alerts', listProjectId || 'all'],
    queryFn: () => api.alerts(listProjectId || undefined),
  })

  const createMutation = useMutation({
    mutationFn: () => {
      const resolvedTarget =
        channel === 'telegram' ? `${botToken.trim()}|${chatId.trim()}` : target
      return api.createAlert({
        project_id: Number(createProjectId),
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
      setFormError('')
      setOpen(false)
      void qc.invalidateQueries({ queryKey: ['alerts'] })
    },
    onError: (e) => setFormError(String(e)),
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
        enableHiding: true,
        meta: {
          label: 'Enabled',
          variant: 'select',
          options: enabledOptions,
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Enabled" />
        ),
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {row.original.enabled ? 'Enabled' : 'Disabled'}
          </span>
        ),
        filterFn: (row, _id, value) => {
          const selected = Array.isArray(value)
            ? value.map(String)
            : [String(value)]
          return selected.includes(String(row.original.enabled))
        },
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
      columnVisibility: { enabled: false },
    },
  })

  const error = rulesQuery.error ? String(rulesQuery.error) : ''

  function onCreate(e: FormEvent) {
    e.preventDefault()
    createMutation.mutate()
  }

  return (
    <section className="flex flex-col gap-4">
      <PageHeader
        title="Alerts"
        description="Notify on issues, volume, and missed crons."
        actions={
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger
              render={
                <Button
                  disabled={!createProjectId}
                  aria-label="Create rule"
                />
              }
            >
              <PlusIcon data-icon="inline-start" />
              <PageHeaderActionLabel>Create rule</PageHeaderActionLabel>
            </DialogTrigger>
            <DialogContent className="sm:max-w-lg">
              <DialogHeader>
                <DialogTitle>Create alert rule</DialogTitle>
                <DialogDescription>
                  Delivered via Slack, email, webhook, or Telegram for the
                  selected project.
                </DialogDescription>
              </DialogHeader>
              <form onSubmit={onCreate} className="flex flex-col gap-4">
                <FieldGroup className="grid gap-3 sm:grid-cols-2">
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
                </FieldGroup>
                {formError && (
                  <Alert variant="destructive">
                    <AlertCircleIcon />
                    <AlertTitle>Create failed</AlertTitle>
                    <AlertDescription>{formError}</AlertDescription>
                  </Alert>
                )}
                <DialogFooter>
                  <Button
                    type="submit"
                    disabled={createMutation.isPending || !createProjectId}
                  >
                    Create
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        }
      />

      {error && (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {rulesQuery.isLoading || projectsQuery.isLoading ? (
        <Skeleton className="h-48 w-full" />
      ) : (
        <DataTable table={table}>
          <ListDataTableFilters table={table} />
        </DataTable>
      )}
    </section>
  )
}
