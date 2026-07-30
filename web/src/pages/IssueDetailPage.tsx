import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, formatTime, parsePayload, type Event, type Frame, type Issue } from '../api'

export default function IssueDetailPage() {
  const { id = '' } = useParams()
  const [issue, setIssue] = useState<Issue | null>(null)
  const [latest, setLatest] = useState<Event | null>(null)
  const [events, setEvents] = useState<Event[]>([])
  const [expanded, setExpanded] = useState(true)
  const [error, setError] = useState('')

  async function load() {
    const [detail, evs] = await Promise.all([api.issue(id), api.events(id)])
    setIssue(detail.issue)
    setLatest(detail.latest_event)
    setEvents(evs)
  }

  useEffect(() => {
    load().catch((e) => setError(String(e)))
  }, [id])

  async function setStatus(status: string) {
    if (!issue) return
    const updated = await api.updateStatus(issue.id, status)
    setIssue(updated)
  }

  if (error) return <p className="error">{error}</p>
  if (!issue) return <p className="muted">Loading…</p>

  const payload = parsePayload(latest)
  const frames: Frame[] = payload?.frames ?? []
  const tags = latest?.tags ?? payload?.tags ?? {}
  const user = {
    id: latest?.user_id ?? payload?.user?.id,
    email: latest?.user_email ?? payload?.user?.email,
  }

  return (
    <section>
      <p className="crumb">
        <Link to="/issues">Issues</Link> / #{issue.id}
      </p>
      <div className="detail-head">
        <div>
          <h1>{issue.title}</h1>
          <p className="mono muted">{issue.culprit || 'No culprit'}</p>
        </div>
        <div className="actions">
          <span className={`badge status-${issue.status}`}>{issue.status}</span>
          {issue.regressed && <span className="badge warn">regressed</span>}
          <button type="button" onClick={() => setStatus('resolved')}>
            Resolve
          </button>
          <button type="button" onClick={() => setStatus('ignored')}>
            Ignore
          </button>
          <button type="button" className="secondary" onClick={() => setStatus('open')}>
            Reopen
          </button>
        </div>
      </div>

      <div className="meta-grid">
        <div>
          <span className="label">Events</span>
          <strong>{issue.count}</strong>
        </div>
        <div>
          <span className="label">First seen</span>
          <strong>{formatTime(issue.first_seen)}</strong>
        </div>
        <div>
          <span className="label">Last seen</span>
          <strong>{formatTime(issue.last_seen)}</strong>
        </div>
        <div>
          <span className="label">Environments</span>
          <strong>{(issue.environments ?? []).join(', ') || '—'}</strong>
        </div>
        <div>
          <span className="label">First release</span>
          <strong>{issue.first_release || '—'}</strong>
        </div>
        <div>
          <span className="label">Last release</span>
          <strong>{issue.last_release || '—'}</strong>
        </div>
      </div>

      <div className="panels">
        <article className="panel">
          <div className="panel-head">
            <h2>Stack trace</h2>
            <button type="button" className="secondary" onClick={() => setExpanded((v) => !v)}>
              {expanded ? 'Collapse' : 'Expand'}
            </button>
          </div>
          {frames.length === 0 && <p className="muted">No stack frames on latest event.</p>}
          {expanded && (
            <ol className="frames">
              {[...frames].reverse().map((f, i) => (
                <li key={i} className={f.in_app ? 'in-app' : ''}>
                  <div className="mono">
                    {f.filename || f.abs_path || f.module || '?'}
                    {f.lineno ? `:${f.lineno}` : ''}
                    {f.function ? ` in ${f.function}` : ''}
                  </div>
                </li>
              ))}
            </ol>
          )}
        </article>

        <article className="panel">
          <h2>Tags</h2>
          <dl className="kv">
            {Object.entries(tags).map(([k, v]) => (
              <div key={k}>
                <dt>{k}</dt>
                <dd>{v}</dd>
              </div>
            ))}
            {Object.keys(tags).length === 0 && <p className="muted">No tags.</p>}
          </dl>
        </article>

        <article className="panel">
          <h2>User</h2>
          <dl className="kv">
            <div>
              <dt>id</dt>
              <dd>{user.id || '—'}</dd>
            </div>
            <div>
              <dt>email</dt>
              <dd>{user.email || '—'}</dd>
            </div>
          </dl>
        </article>
      </div>

      <article className="panel">
        <h2>Event timeline</h2>
        <table className="table">
          <thead>
            <tr>
              <th>Event ID</th>
              <th>Timestamp</th>
              <th>Environment</th>
              <th>Release</th>
            </tr>
          </thead>
          <tbody>
            {events.map((ev) => (
              <tr key={ev.event_id}>
                <td className="mono">{ev.event_id.slice(0, 16)}</td>
                <td>{formatTime(ev.timestamp)}</td>
                <td>{ev.environment || '—'}</td>
                <td>{ev.release || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </article>
    </section>
  )
}
