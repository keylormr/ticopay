import { useState, type FormEvent } from 'react'
import { ApiError, api } from '../api'
import { useI18n } from '../i18n'
import { formatMoney } from '../format'

interface Receipt {
  comprobante: string
  recipientName: string
  amountCents: number
  at: string
}

export function Sinpe({ reload }: { reload: () => Promise<void> }) {
  const { t } = useI18n()
  const [phone, setPhone] = useState('')
  const [amount, setAmount] = useState('')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [receipt, setReceipt] = useState<Receipt | null>(null)

  async function onSend(e: FormEvent) {
    e.preventDefault()
    setError('')
    const digits = phone.replace(/\D/g, '')
    if (digits.length !== 8) {
      setError(t('sinpe.err.phone'))
      return
    }
    const value = Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError(t('sinpe.err.amount'))
      return
    }
    setBusy(true)
    try {
      const r = await api.sinpe({ toPhone: digits, amount: value, description })
      setReceipt(r)
      await reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('sinpe.err.fail'))
    } finally {
      setBusy(false)
    }
  }

  function reset() {
    setReceipt(null)
    setPhone('')
    setAmount('')
    setDescription('')
    setError('')
  }

  if (receipt) {
    return (
      <section className="panel narrow sinpe-receipt">
        <div className="sinpe-check">✓</div>
        <h2>{t('sinpe.ok.title')}</h2>
        <div className="sinpe-amount">{formatMoney(receipt.amountCents, 'CRC')}</div>
        <p className="sub">{t('sinpe.ok.to', { name: receipt.recipientName })}</p>
        <div className="sinpe-ref">
          <span>{t('sinpe.comprobante')}</span>
          <strong>{receipt.comprobante}</strong>
        </div>
        <button className="btn btn-sinpe" onClick={reset}>
          {t('sinpe.another')}
        </button>
      </section>
    )
  }

  return (
    <section className="panel narrow">
      <div className="sinpe-head">
        <span className="sinpe-logo">SINPE</span>
        <h2 style={{ margin: 0 }}>{t('sinpe.title')}</h2>
      </div>
      <p className="sub">{t('sinpe.sub')}</p>
      <form onSubmit={onSend}>
        <label htmlFor="sphone">{t('sinpe.phone')}</label>
        <input id="sphone" type="tel" value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="8888-0000" required />
        <label htmlFor="samt">{t('sinpe.amount')}</label>
        <input id="samt" type="number" min="0" step="any" value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="5000" required />
        <label htmlFor="sdesc">{t('sinpe.detail')}</label>
        <input id="sdesc" value={description} onChange={(e) => setDescription(e.target.value)} placeholder={t('sinpe.detail.ph')} />
        {error && <div className="error">{error}</div>}
        <button className="btn btn-sinpe" type="submit" disabled={busy}>
          {busy ? t('sinpe.busy') : t('sinpe.btn')}
        </button>
      </form>
    </section>
  )
}
