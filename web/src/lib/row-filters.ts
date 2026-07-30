import type { ExtendedColumnFilter, JoinOperator } from '@/types/data-table'
import { getValidFilters } from '@/lib/data-table'

export function firstFilterValue(value: unknown): string {
  if (Array.isArray(value)) {
    const item = value.find((v) => v != null && v !== '')
    return item == null ? '' : String(item)
  }
  if (value == null) return ''
  return String(value)
}

function filterValues(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.filter((v) => v != null && v !== '').map(String)
  }
  if (value == null || value === '') return []
  return String(value)
    .split(',')
    .map((v) => v.trim())
    .filter(Boolean)
}

function matchFilter<T>(
  row: T,
  filter: ExtendedColumnFilter<T>,
  getCell: (row: T, id: string) => unknown
): boolean {
  const raw = getCell(row, filter.id)
  const op = filter.operator
  const selected = filterValues(filter.value)
  const filterValue = firstFilterValue(filter.value)

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
      return selected.some((v) => hay.includes(v))
    }
    if (op === 'notInArray' || op === 'ne') {
      return selected.every((v) => !hay.includes(v))
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
      return selected.includes(text)
    case 'notInArray':
      return !selected.includes(text)
    case 'lt':
    case 'lte':
    case 'gt':
    case 'gte':
    case 'isBetween': {
      const n = typeof raw === 'number' ? raw : Number(raw)
      if (!Number.isFinite(n)) return false
      const a = Number(selected[0])
      const b = Number(selected[1])
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
export function applyAdvancedRowFilters<T>(
  rows: T[],
  filters: ExtendedColumnFilter<T>[],
  joinOperator: JoinOperator = 'and',
  getCell: (row: T, id: string) => unknown = (row, id) =>
    (row as Record<string, unknown>)[id]
): T[] {
  const valid = getValidFilters(filters)
  if (valid.length === 0) return rows

  return rows.filter((row) => {
    const results = valid.map((filter) => matchFilter(row, filter, getCell))
    return joinOperator === 'or'
      ? results.some(Boolean)
      : results.every(Boolean)
  })
}
