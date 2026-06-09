import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, type Biller } from '../api'
import { useI18n } from '../i18n'
import { formatMoney } from '../format'

export function Servicios({ reload }: { reload: () => Promise<void> }) {
  const { t } = useI18n()
  const [billers, setBillers] = useState<Biller[]>([])
  const [selected, setSelected] = useState<Biller | null>(null)
  const [reference, setReference] = useState('')
  const [amount, setAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')

  useEffect(() => {
    api.billers().then((r) => setBillers(r.billers)).catch(() => {})
  }, [])

  function pick(b: Biller) {
    setSelected(b)
    setReference('')
    setAmount('')
    setError('')
    setOk('')
  }

  async function onPay(e: FormEvent) {
    e.preventDefault()
    if (!selected) return
    setError('')
    setOk('')
    const value = Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError(t('serv.err.amount'))
      return
    }
    setBusy(true)
    try {
      await api.payService({ billerId: selected.id, reference: reference.trim(), amount: value, currency: 'CRC' })
      setOk(t('serv.ok', { amount: formatMoney(Math.round(value * 100), 'CRC'), name: selected.name }))
      setReference('')
      setAmount('')
      await reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('serv.err'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid">
      <section className="panel">
        <h2>{t('serv.title')}</h2>
        <p className="sub">{t('serv.sub')}</p>
        <div className="biller-grid">
          {billers.map((b) => (
            <button
              key={b.id}
              type="button"
              className={`biller ${selected?.id === b.id ? 'biller-active' : ''}`}
              onClick={() => pick(b)}
            >
              <span className="biller-icon">{b.icon}</span>
              <span className="biller-name">{b.name}</span>
              <span className="biller-cat">{b.category}</span>
            </button>
          ))}
        </div>
      </section>

      <section className="panel narrow">
        {!selected ? (
          <div className="empty">{t('serv.pick')}</div>
        ) : (
          <>
            <h2>
              {selected.icon} {selected.name}
            </h2>
            <p className="sub">{selected.category}</p>
            <form onSubmit={onPay}>
              <label htmlFor="ref">{selected.refLabel}</label>
              <input id="ref" value={reference} onChange={(e) => setReference(e.target.value)} placeholder={selected.refPlaceholder} required />
              <label htmlFor="samount">{t('serv.amount')}</label>
              <input
                id="samount"
                type="number"
                min="0"
                step="any"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder="15000"
                required
              />
              {error && <div className="error">{error}</div>}
              {ok && <div className="ok">{ok}</div>}
              <button className="btn" type="submit" disabled={busy}>
                {busy ? t('serv.busy') : t('serv.btn')}
              </button>
            </form>
          </>
        )}
      </section>
    </div>
  )
}
