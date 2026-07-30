import type { ColumnFiltersState } from '@tanstack/react-table'
import type { Issue } from '@/api'
import type { ExtendedColumnFilter, JoinOperator } from '@/types/data-table'
import { getValidFilters } from '@/lib/data-table'

function firstValue(value: unknown): string {
  if (Array.isArray(value)) {
    const item = value.find((v) => v != null && v !== '')
    return item == null ? '' : String(item)
  }
  if (value == null) return ''
  return String(value)
}

function values(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.filter((v) => v != null && v !== '').map(String)
  }
  if (value == null || value === '') return []
  return String(value)
    .split(',')
    .map((v) => v.trim())
    .filter(Boolean)
}

function timestampToIso(value: unknown): string {
  if (value == null || value === '') return ''
  const n = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(n)) {
    const d = new Date(String(value))
    return Number.isNaN(d.getTime()) ? '' : d.toISOString()
  }
  return new Date(n).toISOString()
}

/** Map tablecn column / advanced filters onto GET /api/internal/issues params. */
export function columnFiltersToIssueParams(input: {
  mode: 'basic' | 'advanced' | 'command'
  columnFilters: ColumnFiltersState
  advancedFilters: ExtendedColumnFilter<Issue>[]
}): Record<string, string> {
  const params: Record<string, string> = {}

  const apply = (id: string, value: unknown) => {
    switch (id) {
      case 'title': {
        const q = firstValue(value)
        if (q) params.q = q
        break
      }
      case 'project_id': {
        const projectId = firstValue(value)
        if (projectId && projectId !== 'all') params.project_id = projectId
        break
      }
      case 'environment': {
        const environment = firstValue(value)
        if (environment && environment !== 'all') params.environment = environment
        break
      }
      case 'release': {
        const release = firstValue(value)
        if (release && release !== 'all') params.release = release
        break
      }
      case 'tag': {
        const tag = firstValue(value)
        if (tag && tag !== 'all') params.tag = tag
        break
      }
      case 'last_seen': {
        const range = Array.isArray(value) ? value : [value]
        const from = timestampToIso(range[0])
        const to = timestampToIso(range[1])
        if (from) params.from = from
        if (to) params.to = to
        break
      }
      default:
        break
    }
  }

  if (input.mode === 'basic') {
    for (const filter of input.columnFilters) {
      apply(filter.id, filter.value)
    }
    return params
  }

  for (const filter of getValidFilters(input.advancedFilters)) {
    apply(filter.id, filter.value)
  }
  return params
}

function cellValue(issue: Issue, id: string): unknown {
  switch (id) {
    case 'title':
      return issue.title
    case 'status':
      return issue.status
    case 'project_id':
      return String(issue.project_id)
    case 'environment':
      return issue.environments ?? []
    case 'release':
      return issue.last_release ?? issue.first_release ?? ''
    case 'tag':
      return ''
    case 'last_seen':
      return new Date(issue.last_seen).getTime()
    case 'culprit':
      return issue.culprit
    case 'count':
      return issue.count
    case 'owner':
      return issue.assignee ?? ''
    default:
      return (issue as Record<string, unknown>)[id]
  }
}

function matchFilter(issue: Issue, filter: ExtendedColumnFilter<Issue>): boolean {
  const raw = cellValue(issue, filter.id)
  const op = filter.operator
  const filterValues = values(filter.value)
  const filterValue = firstValue(filter.value)

  if (op === 'isEmpty') {
    if (Array.isArray(raw)) return raw.length === 0
    return raw == null || raw === ''
  }
  if (op === 'isNotEmpty') {
    if (Array.isArray(raw)) return raw.length > 0
    return raw != null && raw !== ''
  }

  if (Array.isArray(raw)) {
    const hay = raw.map(String)
    if (op === 'inArray' || op === 'eq') {
      return filterValues.some((v) => hay.includes(v))
    }
    if (op === 'notInArray' || op === 'ne') {
      return filterValues.every((v) => !hay.includes(v))
    }
  }

  const text = raw == null ? '' : String(raw)
  const lower = text.toLowerCase()

  switch (op) {
    case 'iLike':
      return lower.includes(filterValue.toLowerCase())
    case 'notILike':
      return !lower.includes(filterValue.toLowerCase())
    case 'eq':
      return text === filterValue
    case 'ne':
      return text !== filterValue
    case 'inArray':
      return filterValues.includes(text)
    case 'notInArray':
      return !filterValues.includes(text)
    case 'lt':
    case 'lte':
    case 'gt':
    case 'gte':
    case 'isBetween': {
      const n = typeof raw === 'number' ? raw : Number(raw)
      if (!Number.isFinite(n)) return false
      const a = Number(filterValues[0])
      const b = Number(filterValues[1])
      if (op === 'lt') return n < a
      if (op === 'lte') return n <= a
      if (op === 'gt') return n > a
      if (op === 'gte') return n >= a
      if (op === 'isBetween') {
        return n >= Math.min(a, b) && n <= Math.max(a, b)
      }
      return true
    }
    default:
      return true
  }
}

/** Client-side refine for advanced / command filter operators. */
export function applyAdvancedIssueFilters(
  issues: Issue[],
  filters: ExtendedColumnFilter<Issue>[],
  joinOperator: JoinOperator = 'and'
): Issue[] {
  const valid = getValidFilters(filters)
  if (valid.length === 0) return issues

  return issues.filter((issue) => {
    const results = valid.map((filter) => matchFilter(issue, filter))
    return joinOperator === 'or'
      ? results.some(Boolean)
      : results.every(Boolean)
  })
}
