import { useState } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { MenuIcon } from 'lucide-react'
import { ModeToggle } from '@/components/mode-toggle'
import { ProjectSwitcher } from '@/components/project-switcher'
import { Button, buttonVariants } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { clearAdminToken, getAdminToken } from '@/lib/auth'
import { cn } from '@/lib/utils'
import { projectPath, useProjectFilter } from '@/hooks/use-project-filter'

const NAV = [
  { to: '/', label: 'Dashboard', end: true },
  { to: '/projects', label: 'Projects' },
  { to: '/issues', label: 'Issues' },
  { to: '/performance', label: 'Performance' },
  { to: '/crons', label: 'Crons' },
  { to: '/releases', label: 'Releases' },
  { to: '/alerts', label: 'Alerts' },
] as const

function NavItems({
  onNavigate,
  projectId,
}: {
  onNavigate?: () => void
  projectId: string
}) {
  return (
    <>
      {NAV.map((item) => (
        <NavLink
          key={item.to}
          to={projectPath(item.to, projectId)}
          end={'end' in item ? item.end : false}
          onClick={onNavigate}
          className={({ isActive }) =>
            cn(
              buttonVariants({ variant: 'ghost', size: 'sm' }),
              'justify-start',
              isActive ? 'bg-muted text-foreground' : 'text-muted-foreground'
            )
          }
        >
          {item.label}
        </NavLink>
      ))}
    </>
  )
}

export default function RootLayout() {
  const navigate = useNavigate()
  const hasToken = Boolean(getAdminToken())
  const { projectId } = useProjectFilter()
  const [menuOpen, setMenuOpen] = useState(false)

  function logout() {
    clearAdminToken()
    navigate('/login', { replace: true })
  }

  return (
    <div className="flex min-h-svh w-full flex-col gap-6 px-6 py-5 pb-12">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:rounded-md focus:bg-background focus:px-3 focus:py-2"
      >
        Skip to content
      </a>
      <header className="flex flex-col gap-4">
        <div className="flex items-center justify-between gap-4">
          <div className="flex min-w-0 items-center gap-2">
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="md:hidden"
              aria-label="Open menu"
              onClick={() => setMenuOpen(true)}
            >
              <MenuIcon />
            </Button>
            <NavLink
              to={projectPath('/', projectId)}
              className="font-mono text-lg font-bold tracking-tight text-foreground"
            >
              sentry-lite
            </NavLink>
          </div>
          <div className="flex min-w-0 items-center gap-1">
            <nav className="hidden items-center gap-1 md:flex">
              <NavItems projectId={projectId} />
            </nav>
            <ProjectSwitcher className="hidden min-w-36 sm:flex" />
            {hasToken ? (
              <Button variant="ghost" size="sm" onClick={logout}>
                Log out
              </Button>
            ) : null}
            <ModeToggle />
          </div>
        </div>
        <div className="sm:hidden">
          <ProjectSwitcher className="w-full" />
        </div>
        <Separator />
      </header>
      <Sheet open={menuOpen} onOpenChange={setMenuOpen}>
        <SheetContent side="left" className="w-64">
          <SheetHeader>
            <SheetTitle>sentry-lite</SheetTitle>
          </SheetHeader>
          <nav className="flex flex-col gap-1 px-2">
            <NavItems projectId={projectId} onNavigate={() => setMenuOpen(false)} />
          </nav>
        </SheetContent>
      </Sheet>
      <main id="main" className="flex flex-1 flex-col gap-4">
        <Outlet />
      </main>
    </div>
  )
}
