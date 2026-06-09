import { useEffect, useState } from 'react'
import { api, type Transaction } from '../api'
import { formatDate, formatMoney } from '../format'

const ICON: Record<Transaction['direction'], string> = { in: '↓', out: '↑', self: '⇄' }

export function Movimientos({ version }: { version: number }) {
  const [txs, setTxs] = useState<Transaction[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    api
      .transactions()
      .then((r) => setTxs(r.transactions))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [version])

  return (
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
