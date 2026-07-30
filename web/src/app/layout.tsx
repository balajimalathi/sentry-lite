import { NavLink, Outlet } from 'react-router-dom'

export default function RootLayout() {
  return (
    <div className="app">
      <header className="topbar">
        <NavLink to="/" className="brand">
          sentry-lite
        </NavLink>
        <nav>
          <NavLink to="/" end>
            Projects
          </NavLink>
          <NavLink to="/issues">Issues</NavLink>
        </nav>
      </header>
      <main className="main">
        <Outlet />
      </main>
    </div>
  )
}
