import { NavLink, Outlet } from 'react-router-dom'
import { ModeToggle } from '@/components/mode-toggle'
import { buttonVariants } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'

export default function RootLayout() {
  return (
    <div className="mx-auto flex min-h-svh w-full max-w-5xl flex-col gap-6 px-6 py-5 pb-12">
      <header className="flex flex-col gap-4">
        <div className="flex items-center justify-between gap-4">
          <NavLink
            to="/"
            className="font-mono text-lg font-bold tracking-tight text-foreground"
          >
            sentry-lite
          </NavLink>
          <div className="flex items-center gap-1">
            <nav className="flex items-center gap-1">
              <NavLink
                to="/"
                end
                className={({ isActive }) =>
                  cn(
                    buttonVariants({ variant: 'ghost', size: 'sm' }),
                    isActive
                      ? 'bg-muted text-foreground'
                      : 'text-muted-foreground'
                  )
                }
              >
                Projects
              </NavLink>
              <NavLink
                to="/issues"
                className={({ isActive }) =>
                  cn(
                    buttonVariants({ variant: 'ghost', size: 'sm' }),
                    isActive
                      ? 'bg-muted text-foreground'
                      : 'text-muted-foreground'
                  )
                }
              >
                Issues
              </NavLink>
              <NavLink
                to="/performance"
                className={({ isActive }) =>
                  cn(
                    buttonVariants({ variant: 'ghost', size: 'sm' }),
                    isActive
                      ? 'bg-muted text-foreground'
                      : 'text-muted-foreground'
                  )
                }
              >
                Performance
              </NavLink>
              <NavLink
                to="/crons"
                className={({ isActive }) =>
                  cn(
                    buttonVariants({ variant: 'ghost', size: 'sm' }),
                    isActive
                      ? 'bg-muted text-foreground'
                      : 'text-muted-foreground'
                  )
                }
              >
                Crons
              </NavLink>
              <NavLink
                to="/releases"
                className={({ isActive }) =>
                  cn(
                    buttonVariants({ variant: 'ghost', size: 'sm' }),
                    isActive
                      ? 'bg-muted text-foreground'
                      : 'text-muted-foreground'
                  )
                }
              >
                Releases
              </NavLink>
              <NavLink
                to="/alerts"
                className={({ isActive }) =>
                  cn(
                    buttonVariants({ variant: 'ghost', size: 'sm' }),
                    isActive
                      ? 'bg-muted text-foreground'
                      : 'text-muted-foreground'
                  )
                }
              >
                Alerts
              </NavLink>
            </nav>
            <ModeToggle />
          </div>
        </div>
        <Separator />
      </header>
      <main className="flex flex-1 flex-col gap-4">
        <Outlet />
      </main>
    </div>
  )
}
