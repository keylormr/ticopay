import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, type Currency, type Rates } from '../api'
import { useI18n } from '../i18n'
import { CurrencySelect } from '../components/CurrencySelect'
import { CRYPTO, metaOf } from '../currencies'
import { formatMoney } from '../format'

export function Convertir({ reload }: { reload: () => Promise<void> }) {
  const { t } = useI18n()
  const [rates, setRates] = useState<Rates | null>(null)
  const [from, setFrom] = useState<Currency>('USD')
  const [to, setTo] = useState<Currency>('CRC')
  const [amount, setAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    api.rates().then(setRates).catch(() => {})
  }, [])

  async function onConvert(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMsg('')
    if (from === to) {
      setError(t('conv.err.diff'))
      return
    }
    const value = Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError(t('conv.err.amount'))
      return
    }
    setBusy(true)
    try {
      const res = await api.convert({ from, to, amount: value })
      setMsg(t('conv.ok', { from: formatMoney(res.fromCents, from), to: formatMoney(res.toCents, to) }))
      setAmount('')
      await reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('conv.err.fail'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel narrow">
      <h2>{t('conv.title')}</h2>
      <p className="sub">{t('conv.sub')}</p>
      <form onSubmit={onConvert}>
        <div className="cvt-row">
          <div>
            <label htmlFor="from">{t('conv.from')}</label>
            <CurrencySelect id="from" value={from} onChange={setFrom} />
          </div>
          <div>
            <label htmlFor="to">{t('conv.to')}</label>
            <CurrencySelect id="to" value={to} onChange={setTo} />
          </div>
        </div>
        <label htmlFor="cvtAmount">{t('conv.amount', { sym: metaOf(from).symbol })}</label>
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
          {busy ? t('conv.busy') : t('conv.btn')}
        </button>
      </form>

      <div className="rates-box">
        <div className="rates-title">{t('conv.refPrices')}</div>
        <div className="rate-line">
          <span>{t('conv.dollarBccr')}</span>
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
  )
}
