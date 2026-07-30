import { createBrowserRouter, Outlet } from 'react-router-dom'
import { NuqsAdapter } from 'nuqs/adapters/react-router/v7'
import RootLayout from './layout'
import HomePage from './page'
import IssuesPage from './issues/page'
import IssueDetailPage from './issues/[id]/page'
import ReleasesPage from './releases/page'
import AlertsPage from './alerts/page'
import PerformancePage from './performance/page'
import TransactionDetailPage from './performance/[name]/page'
import TracePage from './traces/[traceId]/page'
import CronsPage from './crons/page'

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
    children: [
      {
        path: '/',
        element: <RootLayout />,
        children: [
          { index: true, element: <HomePage /> },
          { path: 'issues', element: <IssuesPage /> },
          { path: 'issues/:id', element: <IssueDetailPage /> },
          { path: 'performance', element: <PerformancePage /> },
          { path: 'performance/:name', element: <TransactionDetailPage /> },
          { path: 'traces/:traceId', element: <TracePage /> },
          { path: 'crons', element: <CronsPage /> },
          { path: 'releases', element: <ReleasesPage /> },
          { path: 'alerts', element: <AlertsPage /> },
        ],
      },
    ],
  },
])
