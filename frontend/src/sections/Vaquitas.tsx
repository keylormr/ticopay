import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, type Currency, type Pool } from '../api'
import { useI18n } from '../i18n'
import { formatMoney } from '../format'
import { ShareCard } from '../components/ShareCard'
import { CurrencySelect } from '../components/CurrencySelect'

export function Vaquitas({ version, reload }: { version: number; reload: () => Promise<void> }) {
  const { t } = useI18n()
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
      setError(t('vaq.err.name'))
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
      setError(err instanceof ApiError ? err.message : t('vaq.err.create'))
    } finally {
      setBusy(false)
    }
  }

  const shareUrl = createdId ? `${window.location.origin}/vaquita/${createdId}` : ''

  return (
    <div className="grid">
      <div className="col">
        <section className="panel">
          <h2>{t('vaq.create')}</h2>
          <p className="sub">{t('vaq.create.sub')}</p>
          <form onSubmit={onCreate}>
            <label htmlFor="vname">{t('vaq.name')}</label>
            <input id="vname" value={name} onChange={(e) => setName(e.target.value)} placeholder={t('vaq.name.ph')} required />
            <label htmlFor="vcur">{t('vaq.currency')}</label>
            <CurrencySelect id="vcur" value={currency} onChange={setCurrency} />
            <label htmlFor="vgoal">{t('vaq.goal')}</label>
            <input id="vgoal" type="number" min="0" step="any" value={goal} onChange={(e) => setGoal(e.target.value)} placeholder="50000" />
            <label htmlFor="vdesc">{t('vaq.desc')}</label>
            <input id="vdesc" value={description} onChange={(e) => setDescription(e.target.value)} placeholder={t('vaq.desc.ph')} />
            {error && <div className="error">{error}</div>}
            <button className="btn" type="submit" disabled={busy}>
              {busy ? t('vaq.busy') : t('vaq.btn')}
            </button>
          </form>
          {shareUrl && (
            <>
              <div className="ok" style={{ marginTop: 16 }}>{t('vaq.created')}</div>
              <ShareCard url={shareUrl} message={t('vaq.shareMsg')} />
            </>
          )}
        </section>
      </div>

      <div className="col">
        <section className="panel">
          <h2>{t('vaq.mine')}</h2>
          {mine.length === 0 && joined.length === 0 && <div className="empty">{t('vaq.empty')}</div>}
          {mine.map((p) => (
            <PoolCard key={p.id} pool={p} reload={async () => { await reload(); load() }} />
          ))}
          {joined.length > 0 && <h2 style={{ marginTop: 20 }}>{t('vaq.joined')}</h2>}
          {joined.map((p) => (
            <PoolCard key={p.id} pool={p} reload={async () => { await reload(); load() }} />
          ))}
        </section>
      </div>
    </div>
  )
}

function PoolCard({ pool, reload }: { pool: Pool; reload: () => Promise<void> }) {
  const { t } = useI18n()
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
      setError(t('vaq.err.amount'))
      return
    }
    setBusy(true)
    try {
      await api.contributePool(pool.id, value)
      setAmount('')
      await reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('vaq.err.contribute'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="pool-card">
      <div className="pool-head">
        <div>
          <div className="name">{pool.name}</div>
          <div className="desc">{pool.description || pool.ownerName}</div>
        </div>
        <div className="pool-raised">{formatMoney(pool.raisedCents, pool.currency)}</div>
      </div>
      {pool.goalCents > 0 && (
        <>
          <div className="bar">
            <div className="bar-fill" style={{ width: `${pct}%` }} />
          </div>
          <div className="bar-label">{t('vaq.goalOf', { pct, goal: formatMoney(pool.goalCents, pool.currency) })}</div>
        </>
      )}
      <div className="pool-actions">
        {pool.isOwner ? (
          <button className="btn-ghost" onClick={() => setShowShare((s) => !s)}>
            {showShare ? t('vaq.hideShare') : t('vaq.share')}
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
              placeholder={t('vaq.contribute.ph', { sym: pool.currency })}
            />
            <button className="btn-pay" onClick={contribute} disabled={busy}>
              {busy ? '…' : t('vaq.contribute')}
            </button>
          </>
        )}
      </div>
      {error && <div className="error" style={{ marginTop: 8 }}>{error}</div>}
      {showShare && <ShareCard url={shareUrl} message={t('vaq.shareMsg')} />}
    </div>
  )
}
