import { useEffect, useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, formatTime, type Issue, type Project } from '../api'

export default function IssuesPage() {
  const [params, setParams] = useSearchParams()
  const [issues, setIssues] = useState<Issue[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [error, setError] = useState('')

  const projectId = params.get('project_id') ?? ''
  const environment = params.get('environment') ?? ''
  const release = params.get('release') ?? ''
  const q = params.get('q') ?? ''

  useEffect(() => {
    api.projects().then(setProjects).catch(() => {})
  }, [])

  useEffect(() => {
    api
      .issues({ project_id: projectId, environment, release, q })
      .then(setIssues)
      .catch((e) => setError(String(e)))
  }, [projectId, environment, release, q])

  function onFilter(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const next = new URLSearchParams()
    ;['project_id', 'environment', 'release', 'q'].forEach((k) => {
      const v = String(fd.get(k) ?? '').trim()
      if (v) next.set(k, v)
    })
    setParams(next)
  }

  if (error) return <p className="error">{error}</p>

  return (
    <section>
      <h1>Issues</h1>
      <form className="filters" onSubmit={onFilter}>
        <label>
          Project
          <select name="project_id" defaultValue={projectId}>
            <option value="">All</option>
            {projects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Environment
          <input name="environment" defaultValue={environment} placeholder="production" />
        </label>
        <label>
          Release
          <input name="release" defaultValue={release} placeholder="1.0.0" />
        </label>
        <label>
          Search
          <input name="q" defaultValue={q} placeholder="title or culprit" />
        </label>
        <button type="submit">Filter</button>
      </form>

      <table className="table">
        <thead>
          <tr>
            <th>Title</th>
            <th>Status</th>
            <th>Count</th>
            <th>First seen</th>
            <th>Last seen</th>
            <th>Culprit</th>
          </tr>
        </thead>
        <tbody>
          {issues.map((iss) => (
            <tr key={iss.id}>
              <td>
                <Link to={`/issues/${iss.id}`}>{iss.title}</Link>
                {iss.regressed && <span className="badge warn">regressed</span>}
              </td>
              <td>
                <span className={`badge status-${iss.status}`}>{iss.status}</span>
              </td>
              <td>{iss.count}</td>
              <td>{formatTime(iss.first_seen)}</td>
              <td>{formatTime(iss.last_seen)}</td>
              <td className="mono muted">{iss.culprit || '—'}</td>
            </tr>
          ))}
          {issues.length === 0 && (
            <tr>
              <td colSpan={6} className="muted">
                No issues match these filters.
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </section>
  )
}
