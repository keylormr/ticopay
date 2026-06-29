import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, type AdminUser, type Capabilities } from '../../api'
import { useI18n } from '../../i18n'

const LIMIT = 25

export function Usuarios({ caps }: { caps: Capabilities }) {
  const { t } = useI18n()
  const [users, setUsers] = useState<AdminUser[]>([])
  const [roles, setRoles] = useState<string[]>([])
  const [total, setTotal] = useState(0)
  const [q, setQ] = useState('')
  const [offset, setOffset] = useState(0)
  const [error, setError] = useState('')

  function load() {
    api
      .adminListUsers({ q, limit: LIMIT, offset })
      .then((r) => {
        setUsers(r.users)
        setRoles(r.roles)
        setTotal(r.total)
      })
      .catch(() => {})
  }
  useEffect(load, [q, offset])

  return (
    <div className="grid">
      {caps.manageUsers && (
        <div className="col">
          <CreateStaff roles={roles} onCreated={load} />
        </div>
      )}
      <div className="col">
        <section className="panel">
          <h2>{t('users.title')}</h2>
          <p className="sub">{t('users.sub', { total })}</p>
          <input
            value={q}
            onChange={(e) => {
              setOffset(0)
              setQ(e.target.value)
            }}
            placeholder={t('users.search.ph')}
          />
          {error && <div className="error">{error}</div>}
          {users.length === 0 && <div className="empty">{t('users.empty')}</div>}
          {users.map((u) => (
            <UserRow key={u.id} u={u} roles={roles} caps={caps} reload={load} onError={setError} />
          ))}
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
      </div>
    </div>
  )
}

function roleLabel(t: (k: string) => string, role: string) {
  const k = `role.${role}`
  const v = t(k)
  return v === k ? role : v
}

function CreateStaff({ roles, onCreated }: { roles: string[]; onCreated: () => void }) {
  const { t } = useI18n()
  const [email, setEmail] = useState('')
  const [fullName, setFullName] = useState('')
  const [phone, setPhone] = useState('')
  const [role, setRole] = useState('support')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setOk('')
    setBusy(true)
    try {
      const u = await api.adminCreateUser({
        email: email.trim(),
        fullName: fullName.trim(),
        phone: phone.trim() || undefined,
        role,
        password,
      })
      setOk(t('users.created', { email: u.email }))
      setEmail('')
      setFullName('')
      setPhone('')
      setPassword('')
      onCreated()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('users.err.create'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel">
      <h2>{t('users.create')}</h2>
      <p className="sub">{t('users.create.sub')}</p>
      <form onSubmit={onSubmit}>
        <label htmlFor="su-name">{t('users.name')}</label>
        <input id="su-name" value={fullName} onChange={(e) => setFullName(e.target.value)} />
        <label htmlFor="su-email">{t('users.email')}</label>
        <input id="su-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="staff@ticopay.cr" />
        <label htmlFor="su-phone">{t('users.phone')}</label>
        <input id="su-phone" value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="8888-0000" />
        <label htmlFor="su-role">{t('users.role')}</label>
        <select id="su-role" value={role} onChange={(e) => setRole(e.target.value)}>
          {roles.map((r) => (
            <option key={r} value={r}>
              {roleLabel(t, r)}
            </option>
          ))}
        </select>
        <label htmlFor="su-pwd">{t('users.password')}</label>
        <input id="su-pwd" type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder={t('users.password.ph')} />
        {error && <div className="error">{error}</div>}
        {ok && <div className="ok">{ok}</div>}
        <button className="btn" type="submit" disabled={busy}>
          {busy ? t('users.creating') : t('users.create.btn')}
        </button>
      </form>
    </section>
  )
}

function UserRow({
  u,
  roles,
  caps,
  reload,
  onError,
}: {
  u: AdminUser
  roles: string[]
  caps: Capabilities
  reload: () => void
  onError: (s: string) => void
}) {
  const { t } = useI18n()
  const [busy, setBusy] = useState(false)

  async function run(fn: () => Promise<unknown>) {
    setBusy(true)
    onError('')
    try {
      await fn()
      reload()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : t('admin.err'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="req-row" style={{ alignItems: 'center' }}>
      <div className="tx-meta">
        <div className="name">
          {u.fullName} {u.disabled && <span className="pill pill-rejected">{t('users.disabled')}</span>}
        </div>
        <div className="desc">
          {u.email} · <span style={{ textTransform: 'capitalize' }}>{roleLabel(t, u.role)}</span>
        </div>
      </div>
      {caps.manageUsers && (
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <select value={u.role} disabled={busy} onChange={(e) => run(() => api.adminSetRole(u.id, e.target.value))}>
            {roles.map((r) => (
              <option key={r} value={r}>
                {roleLabel(t, r)}
              </option>
            ))}
          </select>
          <button className="btn-ghost" disabled={busy} onClick={() => run(() => api.adminSetStatus(u.id, !u.disabled))}>
            {u.disabled ? t('users.enable') : t('users.disable')}
          </button>
        </div>
      )}
    </div>
  )
}
