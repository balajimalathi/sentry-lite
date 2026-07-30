import { PageHeader } from '@/components/page-header'

export default function DashboardPage() {
  return (
    <section className="flex flex-col gap-4">
      <PageHeader
        title="Dashboard"
        description="Overview metrics — coming soon."
      />
      <div className="flex min-h-40 items-center justify-center rounded-lg border border-dashed border-border px-6 py-12">
        <p className="text-sm text-muted-foreground">
          No dashboard widgets yet. Use the nav to browse projects, issues, and
          more.
        </p>
      </div>
    </section>
  )
}
