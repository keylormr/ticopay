import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, type Currency, type ExchangeRate, type Transaction } from '../api'
import { formatDate, formatMoney } from '../format'

const ICON: Record<Transaction['direction'], string> = { in: '↓', out: '↑', self: '⇄' }

export function Home({ version, reload }: { version: number; reload: () => Promise<void> }) {
  const [txs, setTxs] = useState<Transaction[]>([])
  const [rate, setRate] = useState<ExchangeRate | null>(null)
  const [loading, setLoading] = useState(true)

  const [from, setFrom] = useState<Currency>('CRC')
  const [amount, setAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    api.exchangeRate().then(setRate).catch(() => {})
  }, [])

  useEffect(() => {
    setLoading(true)
    api
      .transactions()
      .then((r) => setTxs(r.transactions))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [version])

  const to: Currency = from === 'CRC' ? 'USD' : 'CRC'

  async function onConvert(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMsg('')
    const value = Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError('Ingresá un monto válido')
      return
    }
    setBusy(true)
    try {
      const res = await api.convert({ from, to, amount: value })
      setMsg(`Convertiste ${formatMoney(res.fromCents, from)} → ${formatMoney(res.toCents, to)}`)
      setAmount('')
      await reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'No se pudo convertir')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid">
      <div className="col">
        <section className="panel">
          <h2>Cambio de divisas</h2>
          <p className="sub">
            {rate
              ? `Dólar BCCR · compra ₡${rate.buy.toFixed(2)} · venta ₡${rate.sell.toFixed(2)}`
              : 'Cargando tipo de cambio…'}
          </p>
          <form onSubmit={onConvert}>
            <label htmlFor="from">Convertir desde</label>
            <select id="from" value={from} onChange={(e) => setFrom(e.target.value as Currency)}>
              <option value="CRC">Colones (₡) → Dólares ($)</option>
              <option value="USD">Dólares ($) → Colones (₡)</option>
            </select>
            <label htmlFor="cvtAmount">Monto en {from === 'CRC' ? '₡' : '$'}</label>
            <input
              id="cvtAmount"
              type="number"
              min="0"
              step="0.01"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder={from === 'CRC' ? '10000' : '20'}
              required
            />
            {error && <div className="error">{error}</div>}
            {msg && <div className="ok">{msg}</div>}
            <button className="btn" type="submit" disabled={busy}>
              {busy ? 'Convirtiendo…' : 'Convertir'}
            </button>
          </form>
        </section>
      </div>

      <section className="panel">
        <h2>Movimientos</h2>
        <p className="sub">Tus últimas transacciones en ambas monedas.</p>
        {loading ? (
          <div className="empty">Cargando…</div>
        ) : txs.length === 0 ? (
          <div className="empty">Aún no tenés movimientos.</div>
        ) : (
          <ul className="tx-list">
            {txs.map((t) => (
              <li className="tx-item" key={t.id}>
                <div className={`tx-icon tx-${t.direction}`}>{ICON[t.direction]}</div>
                <div className="tx-meta">
                  <div className="name">{t.direction === 'self' ? t.description || 'Conversión' : t.counterpart}</div>
                  <div className="desc">
                    {labelKind(t)} · {formatDate(t.createdAt)}
                  </div>
                </div>
                <div className={`tx-amount ${t.direction}`}>
                  {t.direction === 'in' ? '+' : t.direction === 'out' ? '−' : ''}
                  {formatMoney(t.amountCents, t.currency)}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}

function labelKind(t: Transaction): string {
  if (t.kind === 'conversion') return 'Conversión'
  if (t.kind === 'pool') return 'Vaquita'
  if (t.kind === 'request') return 'Cobro'
  if (t.description) return t.description
  return t.direction === 'in' ? 'Pago recibido' : 'Pago enviado'
}
