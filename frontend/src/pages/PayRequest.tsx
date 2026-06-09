import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError, api, type PaymentRequest } from '../api'
import { useI18n } from '../i18n'
import { Brand } from '../components/Brand'
import { LangToggle } from '../components/LangToggle'
import { metaOf } from '../currencies'
import { formatMoney } from '../format'

export function PayRequest() {
  const { id = '' } = useParams()
  const { t } = useI18n()
  const [req, setReq] = useState<PaymentRequest | null>(null)
  const [loading, setLoading] = useState(true)
  const [amount, setAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)

  useEffect(() => {
    api
      .getRequest(id)
      .then((r) => setReq(r))
      .catch((e) => setError(e instanceof ApiError ? e.message : t('pay.err.load')))
      .finally(() => setLoading(false))
  }, [id])

  async function pay() {
    setError('')
    const value = req?.amountCents != null ? req.amountCents / 100 : Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError(t('pay.err.amount'))
      return
    }
    setBusy(true)
    try {
      await api.payRequest(id, value)
      setDone(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('pay.err'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth-wrap">
      <div className="card">
        <div className="card-top">
          <Brand />
          <LangToggle />
        </div>
        {loading ? (
          <p className="sub">{t('pay.loading')}</p>
        ) : done ? (
          <>
            <h1>{t('pay.done.title')}</h1>
            <p className="sub">{t('pay.done.sub', { name: req?.requesterName ?? '' })}</p>
            <Link className="btn" to="/">{t('pay.toAccount')}</Link>
          </>
        ) : !req ? (
          <>
            <h1>{t('pay.notFound.title')}</h1>
            <div className="error">{error || t('pay.notFound')}</div>
            <Link className="btn" to="/">{t('pay.toHome')}</Link>
          </>
        ) : req.status !== 'pending' ? (
          <>
            <h1>{t('pay.closed.title')}</h1>
            <p className="sub">{t('pay.closed.sub')}</p>
            <Link className="btn" to="/">{t('pay.toHome')}</Link>
          </>
        ) : (
          <>
            <h1>{t('pay.title', { name: req.requesterName })}</h1>
            <p className="sub">{req.description || t('pay.byTicoPay')}</p>
            <div className="pay-amount">
              {req.amountCents != null ? formatMoney(req.amountCents, req.currency) : t('pay.openAmount')}
            </div>
            {req.amountCents == null && (
              <>
                <label htmlFor="payAmount">{t('pay.amount', { sym: metaOf(req.currency).symbol })}</label>
                <input id="payAmount" type="number" min="0" step="any" value={amount} onChange={(e) => setAmount(e.target.value)} required />
              </>
            )}
            {error && <div className="error">{error}</div>}
            <button className="btn btn-red" onClick={pay} disabled={busy}>
              {busy ? t('pay.busy') : t('pay.btn')}
            </button>
            <div className="switch">
              <Link to="/">{t('pay.back')}</Link>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
