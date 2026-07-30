import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, formatTime, type Project } from '../api'

export default function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    api.projects().then(setProjects).catch((e) => setError(String(e)))
  }, [])

  if (error) return <p className="error">{error}</p>

  return (
    <section>
      <h1>Projects</h1>
      <p className="muted">Recent issue activity per project.</p>
      <table className="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Slug</th>
            <th>Issues</th>
            <th>Latest activity</th>
          </tr>
        </thead>
        <tbody>
          {projects.map((p) => (
            <tr key={p.id}>
              <td>
                <Link to={`/issues?project_id=${p.id}`}>{p.name}</Link>
              </td>
              <td>{p.slug}</td>
              <td>{p.issue_count}</td>
              <td>{formatTime(p.latest_activity_at)}</td>
            </tr>
          ))}
          {projects.length === 0 && (
            <tr>
              <td colSpan={4} className="muted">
                No projects yet.
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </section>
  )
}
