import { createBrowserRouter } from 'react-router-dom'
import RootLayout from './layout'
import HomePage from './page'
import IssuesPage from './issues/page'
import IssueDetailPage from './issues/[id]/page'
import ReleasesPage from './releases/page'
import AlertsPage from './alerts/page'

export const router = createBrowserRouter([
  {
    path: '/',
    element: <RootLayout />,
    children: [
      { index: true, element: <HomePage /> },
      { path: 'issues', element: <IssuesPage /> },
      { path: 'issues/:id', element: <IssueDetailPage /> },
      { path: 'releases', element: <ReleasesPage /> },
      { path: 'alerts', element: <AlertsPage /> },
    ],
  },
])
