import { createBrowserRouter, Outlet } from 'react-router-dom'
import { NuqsAdapter } from 'nuqs/adapters/react-router/v7'
import { RequireAuth } from '@/components/require-auth'
import RootLayout from './layout'
import LoginPage from './login/page'
import DashboardPage from './page'
import ProjectsPage from './projects/page'
import IssuesPage from './issues/page'
import IssueDetailPage from './issues/[id]/page'
import ReleasesPage from './releases/page'
import AlertsPage from './alerts/page'
import PerformancePage from './performance/page'
import TransactionDetailPage from './performance/[name]/page'
import TracePage from './traces/[traceId]/page'
import CronsPage from './crons/page'
import RouteErrorPage, { StandaloneErrorPage } from './error'
import NotFoundPage from './not-found'

function NuqsRoot() {
  return (
    <NuqsAdapter>
      <Outlet />
    </NuqsAdapter>
  )
}

export const router = createBrowserRouter([
  {
    element: <NuqsRoot />,
    errorElement: <StandaloneErrorPage />,
    children: [
      {
        path: '/login',
        element: <LoginPage />,
      },
      {
        element: <RequireAuth />,
        children: [
          {
            path: '/',
            element: <RootLayout />,
            children: [
              {
                errorElement: <RouteErrorPage />,
                children: [
                  { index: true, element: <DashboardPage /> },
                  { path: 'projects', element: <ProjectsPage /> },
                  { path: 'issues', element: <IssuesPage /> },
                  { path: 'issues/:id', element: <IssueDetailPage /> },
                  { path: 'performance', element: <PerformancePage /> },
                  { path: 'performance/:name', element: <TransactionDetailPage /> },
                  { path: 'traces/:traceId', element: <TracePage /> },
                  { path: 'crons', element: <CronsPage /> },
                  { path: 'releases', element: <ReleasesPage /> },
                  { path: 'alerts', element: <AlertsPage /> },
                  { path: '*', element: <NotFoundPage /> },
                ],
              },
            ],
          },
        ],
      },
    ],
  },
])
