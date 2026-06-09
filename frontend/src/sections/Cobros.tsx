import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, type Currency, type PaymentRequest } from '../api'
import { useI18n } from '../i18n'
import { formatMoney } from '../format'
import { ShareCard } from '../components/ShareCard'
import { CurrencySelect } from '../components/CurrencySelect'

export function Cobros({ version, reload }: { version: number; reload: () => Promise<void> }) {
  const { t } = useI18n()
  const [to, setTo] = useState('')
  const [amount, setAmount] = useState('')
  const [currency, setCurrency] = useState<Currency>('CRC')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [createdId, setCreatedId] = useState('')

  const [incoming, setIncoming] = useState<PaymentRequest[]>([])
  const [outgoing, setOutgoing] = useState<PaymentRequest[]>([])

  function load() {
    api
      .listRequests()
      .then((r) => {
        setIncoming(r.incoming)
        setOutgoing(r.outgoing)
      })
      .catch(() => {})
  }
  useEffect(load, [version])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    setError('')
    setCreatedId('')
    setBusy(true)
    try {
      const res = await api.createRequest({
        to: to.trim() || undefined,
        amount: amount ? Number(amount) : undefined,
        currency,
        description: description.trim(),
      })
      setCreatedId(res.id)
      setTo('')
      setAmount('')
      setDescription('')
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('cobros.err.create'))
    } finally {
      setBusy(false)
    }
  }

  const shareUrl = createdId ? `${window.location.origin}/cobro/${createdId}` : ''

  return (
    <div className="grid">
      <div className="col">
        <section className="panel">
          <h2>{t('cobros.create')}</h2>
          <p className="sub">{t('cobros.create.sub')}</p>
          <form onSubmit={onCreate}>
            <label htmlFor="cto">{t('cobros.to')}</label>
            <input id="cto" value={to} onChange={(e) => setTo(e.target.value)} placeholder={t('cobros.to.ph')} />
            <label htmlFor="ccur">{t('cobros.currency')}</label>
            <CurrencySelect id="ccur" value={currency} onChange={setCurrency} />
            <label htmlFor="camount">{t('cobros.amount')}</label>
            <input
              id="camount"
              type="number"
              min="0"
              step="any"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder={t('cobros.amount.ph')}
            />
            <label htmlFor="cdesc">{t('cobros.concept')}</label>
            <input id="cdesc" value={description} onChange={(e) => setDescription(e.target.value)} placeholder={t('cobros.concept.ph')} />
            {error && <div className="error">{error}</div>}
            <button className="btn" type="submit" disabled={busy}>
              {busy ? t('cobros.busy') : t('cobros.btn')}
            </button>
          </form>
          {shareUrl && (
            <>
              <div className="ok" style={{ marginTop: 16 }}>{t('cobros.created')}</div>
              <ShareCard url={shareUrl} message={t('cobros.shareMsg')} />
            </>
          )}
        </section>
      </div>

      <div className="col">
        <section className="panel">
          <h2>{t('cobros.list.title')}</h2>
          <p className="sub">{t('cobros.list.sub')}</p>

          {incoming.length === 0 && outgoing.length === 0 && <div className="empty">{t('cobros.empty')}</div>}

          {incoming.map((r) => (
            <PayRow key={r.id} req={r} reload={async () => { await reload(); load() }} />
          ))}

          {outgoing.map((r) => (
            <div className="req-row" key={r.id}>
              <div className="tx-meta">
                <div className="name">{t('cobros.chargeTo', { name: r.requesterName })}</div>
                <div className="desc">{r.description || t('cobros.noConcept')}</div>
              </div>
              <div className="req-right">
                <div className="req-amount">{r.amountCents != null ? formatMoney(r.amountCents, r.currency) : t('cobros.open')}</div>
                <span className={`pill pill-${r.status}`}>{t(`status.${r.status}`)}</span>
              </div>
            </div>
          ))}
        </section>
      </div>
    </div>
  )
}

function PayRow({ req, reload }: { req: PaymentRequest; reload: () => Promise<void> }) {
  const { t } = useI18n()
  const [amount, setAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function pay() {
    setError('')
    const value = req.amountCents != null ? req.amountCents / 100 : Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError(t('cobros.err.amount'))
      return
    }
    setBusy(true)
    try {
      await api.payRequest(req.id, value)
      await reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('cobros.err.pay'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="req-row">
      <div className="tx-meta">
        <div className="name">{t('cobros.charges', { name: req.requesterName })}</div>
        <div className="desc">{req.description || t('cobros.noConcept')}</div>
        {error && <div className="error" style={{ marginTop: 6 }}>{error}</div>}
      </div>
      <div className="req-right">
        {req.amountCents != null ? (
          <div className="req-amount">{formatMoney(req.amountCents, req.currency)}</div>
        ) : (
          <input
            className="mini-input"
            type="number"
            min="0"
            step="any"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder={req.currency}
          />
        )}
        <button className="btn-pay" onClick={pay} disabled={busy}>
          {busy ? '…' : t('cobros.pay')}
        </button>
      </div>
    </div>
  )
}
