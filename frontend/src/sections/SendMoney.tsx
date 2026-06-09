import { useState, type FormEvent } from 'react'
import { ApiError, api, type Currency } from '../api'
import { useI18n } from '../i18n'
import { CurrencySelect } from '../components/CurrencySelect'
import { metaOf } from '../currencies'
import { formatMoney } from '../format'

export function SendMoney({ reload }: { reload: () => Promise<void> }) {
  const { t } = useI18n()
  const [to, setTo] = useState('')
  const [currency, setCurrency] = useState<Currency>('CRC')
  const [amount, setAmount] = useState('')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')

  const m = metaOf(currency)

  async function onSend(e: FormEvent) {
    e.preventDefault()
    setError('')
    setOk('')
    const value = Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError(t('send.err.amount'))
      return
    }
    setBusy(true)
    try {
      await api.send({ to: to.trim(), amount: value, currency, description })
      setOk(t('send.ok', { amount: formatMoney(Math.round(value * 10 ** m.decimals), currency), to: to.trim() }))
      setTo('')
      setAmount('')
      setDescription('')
      await reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('send.err.fail'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel narrow">
      <h2>{t('send.title')}</h2>
      <p className="sub">{t('send.sub')}</p>
      <form onSubmit={onSend}>
        <label htmlFor="to">{t('send.to')}</label>
        <input id="to" value={to} onChange={(e) => setTo(e.target.value)} placeholder={t('send.to.ph')} required />
        <label htmlFor="currency">{t('send.currency')}</label>
        <CurrencySelect id="currency" value={currency} onChange={setCurrency} />
        <label htmlFor="amount">{t('send.amount', { sym: m.symbol })}</label>
        <input
          id="amount"
          type="number"
          min="0"
          step="any"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          placeholder={m.type === 'crypto' ? '0.01' : '5000'}
          required
        />
        <label htmlFor="desc">{t('send.detail')}</label>
        <input id="desc" value={description} onChange={(e) => setDescription(e.target.value)} placeholder={t('send.detail.ph')} />
        {error && <div className="error">{error}</div>}
        {ok && <div className="ok">{ok}</div>}
        <button className="btn btn-red" type="submit" disabled={busy}>
          {busy ? t('send.busy') : t('send.btn')}
        </button>
      </form>
    </section>
  )
}
