import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError, api, type Pool, type PoolContribution } from '../api'
import { Brand } from '../components/Brand'
import { formatDate, formatMoney } from '../format'

export function ContributePool() {
  const { id = '' } = useParams()
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
      .catch((e) => setError(e instanceof ApiError ? e.message : 'No se pudo cargar la vaquita'))
      .finally(() => setLoading(false))
  }
  useEffect(load, [id])

  async function contribute() {
    setError('')
    setOk('')
    const value = Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError('Indicá un monto')
      return
    }
    setBusy(true)
    try {
      await api.contributePool(id, value)
      setOk(`¡Gracias por tu aporte de ${formatMoney(Math.round(value * 100), pool!.currency)}!`)
      setAmount('')
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'No se pudo aportar')
    } finally {
      setBusy(false)
    }
  }

  const pct = pool && pool.goalCents > 0 ? Math.min(100, Math.round((pool.raisedCents / pool.goalCents) * 100)) : 0

  return (
    <div className="auth-wrap">
      <div className="card">
        <Brand />
        {loading ? (
          <p className="sub">Cargando vaquita…</p>
        ) : !pool ? (
          <>
            <h1>Vaquita no disponible</h1>
            <div className="error">{error || 'No encontramos esta vaquita.'}</div>
            <Link className="btn" to="/">Ir al inicio</Link>
          </>
        ) : (
          <>
            <h1>{pool.name} 🐮</h1>
            <p className="sub">{pool.description || `Organizada por ${pool.ownerName}`}</p>
            <div className="pay-amount">{formatMoney(pool.raisedCents, pool.currency)}</div>
            {pool.goalCents > 0 && (
              <>
                <div className="bar">
                  <div className="bar-fill" style={{ width: `${pct}%` }} />
                </div>
                <div className="bar-label">
                  {pct}% de {formatMoney(pool.goalCents, pool.currency)}
                </div>
              </>
            )}

            {pool.isOwner ? (
              <div className="hint" style={{ marginTop: 16 }}>Es tu vaquita: compartí el enlace para recibir aportes.</div>
            ) : (
              <>
                <label htmlFor="contribAmount">Tu aporte ({pool.currency === 'CRC' ? '₡' : '$'})</label>
                <input
                  id="contribAmount"
                  type="number"
                  min="0"
                  step="0.01"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                />
                {error && <div className="error">{error}</div>}
                {ok && <div className="ok">{ok}</div>}
                <button className="btn" onClick={contribute} disabled={busy}>
                  {busy ? 'Aportando…' : 'Aportar'}
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
              <Link to="/">Ir a mi cuenta</Link>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
