import { useEffect, useState } from 'react'
import { api, type AuditEntry } from '../../api'
import { useI18n } from '../../i18n'

const LIMIT = 25

// Known back-office actions, kept in sync with auditAction in audit.go. Used to
// populate the filter; unknown actions still render via the raw string.
const ACTIONS = [
  'merchant.verify',
  'merchant.reject',
  'merchant.commission',
  'user.create',
  'user.set_role',
  'user.set_status',
]

// labelFor returns a translation if one exists, else the raw key (so a new
// action shipped on the backend before its copy still shows something sensible).
function labelFor(t: (k: string) => string, key: string) {
  const v = t(key)
  return v === key ? key : v
}

function roleLabel(t: (k: string) => string, role: string) {
  return labelFor(t, `role.${role}`)
}

// describeDetail renders the action-specific JSON detail as a short human line.
function describeDetail(t: (k: string) => string, action: string, detail: Record<string, unknown>): string {
  switch (action) {
    case 'merchant.reject':
      return detail.reason ? String(detail.reason) : ''
    case 'merchant.commission':
      return detail.bps != null ? `${detail.bps} bps` : ''
    case 'user.create':
      return [detail.email ? String(detail.email) : '', detail.role ? roleLabel(t, String(detail.role)) : '']
        .filter(Boolean)
        .join(' · ')
    case 'user.set_role':
      return `${roleLabel(t, String(detail.previous ?? '?'))} → ${roleLabel(t, String(detail.role ?? '?'))}`
    case 'user.set_status':
      return detail.disabled ? t('audit.detail.disabled') : t('audit.detail.enabled')
    default:
      return ''
  }
}

export function Auditoria() {
  const { t } = useI18n()
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [total, setTotal] = useState(0)
  const [action, setAction] = useState('')
  const [offset, setOffset] = useState(0)
  const [error, setError] = useState('')

  useEffect(() => {
    api
      .adminAuditLog({ action: action || undefined, limit: LIMIT, offset })
      .then((r) => {
        setEntries(r.entries)
        setTotal(r.total)
      })
      .catch(() => setError(t('admin.err')))
  }, [action, offset])

  return (
    <section className="panel">
      <h2>{t('audit.title')}</h2>
      <p className="sub">{t('audit.sub')}</p>

      <label htmlFor="audit-action">{t('audit.filter')}</label>
      <select
        id="audit-action"
        value={action}
        onChange={(e) => {
          setOffset(0)
          setAction(e.target.value)
        }}
      >
        <option value="">{t('audit.filter.all')}</option>
        {ACTIONS.map((a) => (
          <option key={a} value={a}>
            {labelFor(t, `audit.action.${a}`)}
          </option>
        ))}
      </select>

      {error && <div className="error">{error}</div>}
      {entries.length === 0 && <div className="empty">{t('audit.empty')}</div>}

      {entries.map((e) => {
        const detail = describeDetail(t, e.action, e.detail || {})
        return (
          <div key={e.id} className="req-row" style={{ alignItems: 'center' }}>
            <div className="tx-meta">
              <div className="name">
                {labelFor(t, `audit.action.${e.action}`)}
                {detail && <span className="desc"> — {detail}</span>}
              </div>
              <div className="desc">
                {t('audit.by')} {e.actorName || e.actorEmail || t('audit.unknownActor')} · {new Date(e.createdAt).toLocaleString()}
              </div>
            </div>
          </div>
        )
      })}

      {total > LIMIT && (
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 12 }}>
          <button className="btn-ghost" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - LIMIT))}>
            {t('users.prev')}
          </button>
          <span className="sub">
            {offset + 1}–{Math.min(offset + LIMIT, total)} / {total}
          </span>
          <button className="btn-ghost" disabled={offset + LIMIT >= total} onClick={() => setOffset(offset + LIMIT)}>
            {t('users.next')}
          </button>
        </div>
      )}
    </section>
  )
}
