import { useEffect, useState, type FormEvent } from 'react'
import { startRegistration } from '@simplewebauthn/browser'
import { ApiError, api } from '../api'
import { useAuth } from '../auth'
import { useI18n } from '../i18n'
import { formatDate } from '../format'

const ID_TYPES = ['fisica', 'juridica', 'dimex'] as const

export function Account() {
  const { user, setUser } = useAuth()
  const { t } = useI18n()
  const [idType, setIdType] = useState('fisica')
  const [idNumber, setIdNumber] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const verified = user?.kycStatus === 'verified'

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const res = await api.submitKyc({ idType, idNumber })
      if (user) setUser({ ...user, kycStatus: 'verified', idType: res.idType, idNumber: res.idNumber })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('acct.verify.err'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <section className="panel narrow">
        <h2>{t('acct.title')}</h2>
        <p className="sub">
          {user?.fullName} · {user?.email}
          {user?.phone ? ` · ${user.phone}` : ''}
        </p>

        {verified ? (
          <div className="ok">
            {t('acct.verified')}
            {user?.idNumber ? ` · ${user.idType} ${user.idNumber}` : ''}
          </div>
        ) : (
          <>
            <h2 style={{ marginTop: 18 }}>{t('acct.verify')}</h2>
            <p className="sub">{t('acct.verify.sub')}</p>
            <form onSubmit={onSubmit}>
              <label htmlFor="idType">{t('acct.idType')}</label>
              <select id="idType" value={idType} onChange={(e) => setIdType(e.target.value)}>
                {ID_TYPES.map((v) => (
                  <option key={v} value={v}>
                    {t(`acct.idType.${v}`)}
                  </option>
                ))}
              </select>
              <label htmlFor="idNumber">{t('acct.idNumber')}</label>
              <input id="idNumber" value={idNumber} onChange={(e) => setIdNumber(e.target.value)} placeholder="1-2345-6789" required />
              {error && <div className="error">{error}</div>}
              <button className="btn" type="submit" disabled={busy}>
                {busy ? t('acct.verify.busy') : t('acct.verify.btn')}
              </button>
            </form>
          </>
        )}
      </section>

      <Passkeys />
    </>
  )
}

function Passkeys() {
  const { t } = useI18n()
  const [list, setList] = useState<{ id: string; name: string; createdAt: string }[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')

  function load() {
    api
      .listPasskeys()
      .then((r) => setList(r.passkeys))
      .catch(() => {})
  }
  useEffect(load, [])

  async function add() {
    setError('')
    setOk('')
    setBusy(true)
    try {
      const begin = await api.passkeyRegisterBegin()
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const credential = await startRegistration({ optionsJSON: begin.publicKey as any })
      await api.passkeyRegisterFinish({ sessionToken: begin.sessionToken, credential, name: 'Mi dispositivo' })
      setOk(t('pk.ok'))
      load()
    } catch (err) {
      if (err instanceof ApiError) setError(err.message)
      else if (err instanceof Error && /abort|cancel|NotAllowed/i.test(err.name + err.message)) setError(t('pk.err.cancel'))
      else setError(t('pk.err'))
    } finally {
      setBusy(false)
    }
  }

  async function remove(id: string) {
    try {
      await api.deletePasskey(id)
      load()
    } catch {
      /* ignore */
    }
  }

  return (
    <section className="panel narrow" style={{ marginTop: 18 }}>
      <h2>{t('pk.title')}</h2>
      <p className="sub">{t('pk.sub')}</p>

      {list.length === 0 ? (
        <div className="empty">{t('pk.empty')}</div>
      ) : (
        <ul className="tx-list">
          {list.map((p) => (
            <li className="tx-item" key={p.id}>
              <div className="tx-icon tx-in">🔑</div>
              <div className="tx-meta">
                <div className="name">{p.name}</div>
                <div className="desc">{t('pk.added', { date: formatDate(p.createdAt) })}</div>
              </div>
              <button className="btn-ghost" onClick={() => remove(p.id)}>
                {t('pk.remove')}
              </button>
            </li>
          ))}
        </ul>
      )}

      {error && <div className="error">{error}</div>}
      {ok && <div className="ok">{ok}</div>}
      <button className="btn" onClick={add} disabled={busy}>
        {busy ? t('pk.busy') : t('pk.add')}
      </button>
    </section>
  )
}
