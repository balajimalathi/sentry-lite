import { parseAsArrayOf, parseAsString, useQueryState } from 'nuqs'
import type { Project } from '@/api'
import { firstFilterValue } from '@/lib/row-filters'

export const ALL_PROJECTS = 'all'

/** Stable fallback so `projectsQuery.data ?? []` is not a new array every render. */
export const EMPTY_PROJECTS: Project[] = []

export function useProjectFilter() {
  const [projectIds, setProjectIds] = useQueryState(
    'project_id',
    parseAsArrayOf(parseAsString, ',')
  )
  const projectId = firstFilterValue(projectIds)

  function setProjectId(value: string | null) {
    if (!value || value === ALL_PROJECTS) {
      void setProjectIds(null)
      return
    }
    void setProjectIds([value])
  }

  return { projectId, projectIds: projectIds ?? [], setProjectId }
}

export function projectPath(path: string, projectId: string) {
  if (!projectId) return path
  const url = new URL(path, 'http://local.invalid')
  url.searchParams.set('project_id', projectId)
  return `${url.pathname}${url.search}`
}
