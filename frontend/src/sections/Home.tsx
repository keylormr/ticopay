import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, type Currency, type Rates, type Transaction } from '../api'
import { CurrencySelect } from '../components/CurrencySelect'
import { CRYPTO, metaOf } from '../currencies'
import { formatDate, formatMoney } from '../format'

const ICON: Record<Transaction['direction'], string> = { in: '↓', out: '↑', self: '⇄' }

export function Home({ version, reload }: { version: number; reload: () => Promise<void> }) {
  const [txs, setTxs] = useState<Transaction[]>([])
  const [rates, setRates] = useState<Rates | null>(null)
  const [loading, setLoading] = useState(true)

  const [from, setFrom] = useState<Currency>('USD')
  const [to, setTo] = useState<Currency>('CRC')
  const [amount, setAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    api.rates().then(setRates).catch(() => {})
  }, [])

  useEffect(() => {
    setLoading(true)
    api
      .transactions()
      .then((r) => setTxs(r.transactions))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [version])

  async function onConvert(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMsg('')
    if (from === to) {
      setError('Elegí dos monedas distintas')
      return
    }
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
          <h2>Convertir</h2>
          <p className="sub">Entre colones, dólares y cripto, al instante.</p>
          <form onSubmit={onConvert}>
            <div className="cvt-row">
              <div>
                <label htmlFor="from">De</label>
                <CurrencySelect id="from" value={from} onChange={setFrom} />
              </div>
              <div>
                <label htmlFor="to">A</label>
                <CurrencySelect id="to" value={to} onChange={setTo} />
              </div>
            </div>
            <label htmlFor="cvtAmount">Monto en {metaOf(from).symbol}</label>
            <input
              id="cvtAmount"
              type="number"
              min="0"
              step="any"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder={metaOf(from).type === 'crypto' ? '0.01' : '10000'}
              required
            />
            {error && <div className="error">{error}</div>}
            {msg && <div className="ok">{msg}</div>}
            <button className="btn" type="submit" disabled={busy}>
              {busy ? 'Convirtiendo…' : 'Convertir'}
            </button>
          </form>

          <div className="rates-box">
            <div className="rates-title">Precios de referencia</div>
            <div className="rate-line">
              <span>Dólar (BCCR)</span>
              <span>{rates?.crc?.sell ? `₡${rates.crc.sell.toFixed(2)}` : '—'}</span>
            </div>
            {CRYPTO.slice(0, 6).map((c) => (
              <div className="rate-line" key={c.code}>
                <span>
                  {c.name} ({c.code})
                </span>
                <span>{rates?.crypto?.[c.code] != null ? `$${rates.crypto[c.code].toLocaleString('en-US')}` : '—'}</span>
              </div>
            ))}
          </div>
        </section>
      </div>

      <section className="panel">
        <h2>Movimientos</h2>
        <p className="sub">Tus últimas transacciones, en todas las monedas.</p>
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
                  <div className="name">{txTitle(t)}</div>
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

function txTitle(t: Transaction): string {
  if (t.kind === 'service') return t.description || 'Servicio'
  if (t.direction === 'self') return t.description || 'Conversión'
  return t.counterpart
}

function labelKind(t: Transaction): string {
  if (t.kind === 'conversion') return 'Conversión'
  if (t.kind === 'pool') return 'Vaquita'
  if (t.kind === 'request') return 'Cobro'
  if (t.kind === 'service') return 'Servicio'
  if (t.description) return t.description
  return t.direction === 'in' ? 'Pago recibido' : 'Pago enviado'
}
