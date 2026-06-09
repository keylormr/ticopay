import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError, api, type Pool, type PoolContribution } from '../api'
import { useI18n } from '../i18n'
import { Brand } from '../components/Brand'
import { LangToggle } from '../components/LangToggle'
import { metaOf } from '../currencies'
import { formatDate, formatMoney } from '../format'

export function ContributePool() {
  const { id = '' } = useParams()
  const { t } = useI18n()
  const [pool, setPool] = useState<Pool | null>(null)
  const [contribs, setContribs] = useState<PoolContribution[]>([])
  const [loading, setLoading] = useState(true)
  const [amount, setAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')

  function load() {
    api
      .getPool(id)
      .then((r) => {
        setPool(r.pool)
        setContribs(r.contributions)
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : t('pool.err.load')))
      .finally(() => setLoading(false))
  }
  useEffect(load, [id])

  async function contribute() {
    setError('')
    setOk('')
    const value = Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError(t('pool.err.amount'))
      return
    }
    setBusy(true)
    try {
      await api.contributePool(id, value)
      setOk(t('pool.thanks', { amount: formatMoney(Math.round(value * 100), pool!.currency) }))
      setAmount('')
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('pool.err'))
    } finally {
      setBusy(false)
    }
  }

  const pct = pool && pool.goalCents > 0 ? Math.min(100, Math.round((pool.raisedCents / pool.goalCents) * 100)) : 0

  return (
    <div className="auth-wrap">
      <div className="card">
        <div className="card-top">
          <Brand />
          <LangToggle />
        </div>
        {loading ? (
          <p className="sub">{t('pool.loading')}</p>
        ) : !pool ? (
          <>
            <h1>{t('pool.notFound.title')}</h1>
            <div className="error">{error || t('pool.notFound')}</div>
            <Link className="btn" to="/">{t('pay.toHome')}</Link>
          </>
        ) : (
          <>
            <h1>{pool.name} 🐮</h1>
            <p className="sub">{pool.description || t('pool.org', { name: pool.ownerName })}</p>
            <div className="pay-amount">{formatMoney(pool.raisedCents, pool.currency)}</div>
            {pool.goalCents > 0 && (
              <>
                <div className="bar">
                  <div className="bar-fill" style={{ width: `${pct}%` }} />
                </div>
                <div className="bar-label">{t('pool.goalOf', { pct, goal: formatMoney(pool.goalCents, pool.currency) })}</div>
              </>
            )}

            {pool.isOwner ? (
              <div className="hint" style={{ marginTop: 16 }}>{t('pool.owner')}</div>
            ) : (
              <>
                <label htmlFor="contribAmount">{t('pool.your', { sym: metaOf(pool.currency).symbol })}</label>
                <input id="contribAmount" type="number" min="0" step="any" value={amount} onChange={(e) => setAmount(e.target.value)} />
                {error && <div className="error">{error}</div>}
                {ok && <div className="ok">{ok}</div>}
                <button className="btn" onClick={contribute} disabled={busy}>
                  {busy ? t('pool.busy') : t('pool.contribute')}
                </button>
              </>
            )}

            {contribs.length > 0 && (
              <ul className="tx-list" style={{ marginTop: 18 }}>
                {contribs.map((c, i) => (
                  <li className="tx-item" key={i}>
                    <div className="tx-icon tx-in">🐮</div>
                    <div className="tx-meta">
                      <div className="name">{c.name}</div>
                      <div className="desc">{formatDate(c.createdAt)}</div>
                    </div>
                    <div className="tx-amount in">{formatMoney(c.amountCents, pool.currency)}</div>
                  </li>
                ))}
              </ul>
            )}
            <div className="switch">
              <Link to="/">{t('pay.toAccount')}</Link>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
