import type { ColumnFiltersState } from '@tanstack/react-table'
import type { Issue } from '@/api'
import type { ExtendedColumnFilter, JoinOperator } from '@/types/data-table'
import { getValidFilters } from '@/lib/data-table'
import {
  applyAdvancedRowFilters,
  firstFilterValue,
} from '@/lib/row-filters'

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
        const q = firstFilterValue(value)
        if (q) params.q = q
        break
      }
      case 'project_id': {
        const projectId = firstFilterValue(value)
        if (projectId && projectId !== 'all') params.project_id = projectId
        break
      }
      case 'environment': {
        const environment = firstFilterValue(value)
        if (environment && environment !== 'all') params.environment = environment
        break
      }
      case 'release': {
        const release = firstFilterValue(value)
        if (release && release !== 'all') params.release = release
        break
      }
      case 'tag': {
        const tag = firstFilterValue(value)
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

function issueCellValue(issue: Issue, id: string): unknown {
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

/** Client-side refine for advanced / command filter operators. */
export function applyAdvancedIssueFilters(
  issues: Issue[],
  filters: ExtendedColumnFilter<Issue>[],
  joinOperator: JoinOperator = 'and'
): Issue[] {
  return applyAdvancedRowFilters(issues, filters, joinOperator, issueCellValue)
}
