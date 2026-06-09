import { useEffect, useState } from 'react'
import { api, type Transaction } from '../api'
import { useI18n } from '../i18n'
import { formatDate, formatMoney } from '../format'

const ICON: Record<Transaction['direction'], string> = { in: '↓', out: '↑', self: '⇄' }

export function Movimientos({ version }: { version: number }) {
  const { t } = useI18n()
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

  function txTitle(tx: Transaction): string {
    if (tx.kind === 'service') return tx.description || t('mov.servicio')
    if (tx.direction === 'self') return tx.description || t('mov.conversion')
    return tx.counterpart
  }

  function labelKind(tx: Transaction): string {
    if (tx.kind === 'conversion') return t('mov.conversion')
    if (tx.kind === 'pool') return t('mov.vaquita')
    if (tx.kind === 'request') return t('mov.cobro')
    if (tx.kind === 'service') return t('mov.servicio')
    if (tx.description) return tx.description
    return tx.direction === 'in' ? t('mov.received') : t('mov.sent')
  }

  return (
    <section className="panel">
      <h2>{t('mov.title')}</h2>
      <p className="sub">{t('mov.sub')}</p>
      {loading ? (
        <div className="empty">{t('common.loading')}</div>
      ) : txs.length === 0 ? (
        <div className="empty">{t('mov.empty')}</div>
      ) : (
        <ul className="tx-list">
          {txs.map((tx) => (
            <li className="tx-item" key={tx.id}>
              <div className={`tx-icon tx-${tx.direction}`}>{ICON[tx.direction]}</div>
              <div className="tx-meta">
                <div className="name">{txTitle(tx)}</div>
                <div className="desc">
                  {labelKind(tx)} · {formatDate(tx.createdAt)}
                </div>
              </div>
              <div className={`tx-amount ${tx.direction}`}>
                {tx.direction === 'in' ? '+' : tx.direction === 'out' ? '−' : ''}
                {formatMoney(tx.amountCents, tx.currency)}
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
