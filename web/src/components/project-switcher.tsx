import { useQuery } from '@tanstack/react-query'
import { api } from '@/api'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  ALL_PROJECTS,
  useProjectFilter,
} from '@/hooks/use-project-filter'

export function ProjectSwitcher({ className }: { className?: string }) {
  const { projectId, setProjectId } = useProjectFilter()
  const projectsQuery = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.projects(),
  })
  const projects = projectsQuery.data ?? []
  const items = [
    { label: 'All projects', value: ALL_PROJECTS },
    ...projects.map((p) => ({ label: p.name, value: String(p.id) })),
  ]

  return (
    <Select
      items={items}
      value={projectId || ALL_PROJECTS}
      onValueChange={(v) => setProjectId(v == null ? null : String(v))}
    >
      <SelectTrigger size="sm" className={className ?? 'min-w-40'}>
        <SelectValue placeholder="All projects" />
      </SelectTrigger>
      <SelectContent align="end">
        <SelectGroup>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}
