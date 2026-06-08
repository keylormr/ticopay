import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, type Currency, type Pool } from '../api'
import { formatMoney } from '../format'
import { ShareCard } from '../components/ShareCard'
import { CurrencySelect } from '../components/CurrencySelect'

export function Vaquitas({ version, reload }: { version: number; reload: () => Promise<void> }) {
  const [name, setName] = useState('')
  const [goal, setGoal] = useState('')
  const [currency, setCurrency] = useState<Currency>('CRC')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [createdId, setCreatedId] = useState('')

  const [mine, setMine] = useState<Pool[]>([])
  const [joined, setJoined] = useState<Pool[]>([])

  function load() {
    api
      .listPools()
      .then((r) => {
        setMine(r.mine)
        setJoined(r.joined)
      })
      .catch(() => {})
  }
  useEffect(load, [version])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    setError('')
    setCreatedId('')
    if (!name.trim()) {
      setError('Poné un nombre a la vaquita')
      return
    }
    setBusy(true)
    try {
      const res = await api.createPool({
        name: name.trim(),
        description: description.trim(),
        goalAmount: goal ? Number(goal) : 0,
        currency,
      })
      setCreatedId(res.id)
      setName('')
      setGoal('')
      setDescription('')
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'No se pudo crear la vaquita')
    } finally {
      setBusy(false)
    }
  }

  const shareUrl = createdId ? `${window.location.origin}/vaquita/${createdId}` : ''

  return (
    <div className="grid">
      <div className="col">
        <section className="panel">
          <h2>Crear una vaquita 🐮</h2>
          <p className="sub">Juntá plata en grupo: regalo, paseo, lo que sea. Compartí el enlace.</p>
          <form onSubmit={onCreate}>
            <label htmlFor="vname">Nombre</label>
            <input id="vname" value={name} onChange={(e) => setName(e.target.value)} placeholder="Cumpleaños de Ana" required />
            <label htmlFor="vcur">Moneda</label>
            <CurrencySelect id="vcur" value={currency} onChange={setCurrency} />
            <label htmlFor="vgoal">Meta (opcional)</label>
            <input id="vgoal" type="number" min="0" step="any" value={goal} onChange={(e) => setGoal(e.target.value)} placeholder="50000" />
            <label htmlFor="vdesc">Descripción</label>
            <input id="vdesc" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Para el queque y el regalo" />
            {error && <div className="error">{error}</div>}
            <button className="btn" type="submit" disabled={busy}>
              {busy ? 'Creando…' : 'Crear vaquita'}
            </button>
          </form>
          {shareUrl && (
            <>
              <div className="ok" style={{ marginTop: 16 }}>¡Vaquita creada! Invitá a aportar:</div>
              <ShareCard url={shareUrl} message="¡Aportá a esta vaquita en Tico Pay!" />
            </>
          )}
        </section>
      </div>

      <div className="col">
        <section className="panel">
          <h2>Mis vaquitas</h2>
          {mine.length === 0 && joined.length === 0 && <div className="empty">Todavía no tenés vaquitas.</div>}
          {mine.map((p) => (
            <PoolCard key={p.id} pool={p} reload={async () => { await reload(); load() }} />
          ))}
          {joined.length > 0 && <h2 style={{ marginTop: 20 }}>Donde aporté</h2>}
          {joined.map((p) => (
            <PoolCard key={p.id} pool={p} reload={async () => { await reload(); load() }} />
          ))}
        </section>
      </div>
    </div>
  )
}

function PoolCard({ pool, reload }: { pool: Pool; reload: () => Promise<void> }) {
  const [amount, setAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [showShare, setShowShare] = useState(false)

  const pct = pool.goalCents > 0 ? Math.min(100, Math.round((pool.raisedCents / pool.goalCents) * 100)) : 0
  const shareUrl = `${window.location.origin}/vaquita/${pool.id}`

  async function contribute() {
    setError('')
    const value = Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError('Indicá un monto')
      return
    }
    setBusy(true)
    try {
      await api.contributePool(pool.id, value)
      setAmount('')
      await reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'No se pudo aportar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="pool-card">
      <div className="pool-head">
        <div>
          <div className="name">{pool.name}</div>
          <div className="desc">{pool.description || `de ${pool.ownerName}`}</div>
        </div>
        <div className="pool-raised">{formatMoney(pool.raisedCents, pool.currency)}</div>
      </div>
      {pool.goalCents > 0 && (
        <>
          <div className="bar">
            <div className="bar-fill" style={{ width: `${pct}%` }} />
          </div>
          <div className="bar-label">
            {pct}% de la meta {formatMoney(pool.goalCents, pool.currency)}
          </div>
        </>
      )}
      <div className="pool-actions">
        {pool.isOwner ? (
          <button className="btn-ghost" onClick={() => setShowShare((s) => !s)}>
            {showShare ? 'Ocultar enlace' : 'Compartir'}
          </button>
        ) : (
          <>
            <input
              className="mini-input"
              type="number"
              min="0"
              step="any"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder={`${pool.currency} aporte`}
            />
            <button className="btn-pay" onClick={contribute} disabled={busy}>
              {busy ? '…' : 'Aportar'}
            </button>
          </>
        )}
      </div>
      {error && <div className="error" style={{ marginTop: 8 }}>{error}</div>}
      {showShare && <ShareCard url={shareUrl} message={`Aportá a "${pool.name}" en Tico Pay:`} />}
    </div>
  )
}
