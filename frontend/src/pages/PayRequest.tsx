import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError, api, type PaymentRequest } from '../api'
import { Brand } from '../components/Brand'
import { formatMoney } from '../format'

export function PayRequest() {
  const { id = '' } = useParams()
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
      .catch((e) => setError(e instanceof ApiError ? e.message : 'No se pudo cargar el cobro'))
      .finally(() => setLoading(false))
  }, [id])

  async function pay() {
    setError('')
    const value = req?.amountCents != null ? req.amountCents / 100 : Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError('Indicá un monto')
      return
    }
    setBusy(true)
    try {
      await api.payRequest(id, value)
      setDone(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'No se pudo pagar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth-wrap">
      <div className="card">
        <Brand />
        {loading ? (
          <p className="sub">Cargando cobro…</p>
        ) : done ? (
          <>
            <h1>¡Pago realizado! ✅</h1>
            <p className="sub">Le pagaste a {req?.requesterName}.</p>
            <Link className="btn" to="/">Ir a mi cuenta</Link>
          </>
        ) : !req ? (
          <>
            <h1>Cobro no disponible</h1>
            <div className="error">{error || 'No encontramos este cobro.'}</div>
            <Link className="btn" to="/">Ir al inicio</Link>
          </>
        ) : req.status !== 'pending' ? (
          <>
            <h1>Cobro ya cerrado</h1>
            <p className="sub">Este cobro ya fue pagado o cancelado.</p>
            <Link className="btn" to="/">Ir al inicio</Link>
          </>
        ) : (
          <>
            <h1>Pagar a {req.requesterName}</h1>
            <p className="sub">{req.description || 'Cobro por Tico Pay'}</p>
            <div className="pay-amount">
              {req.amountCents != null ? formatMoney(req.amountCents, req.currency) : 'Monto abierto'}
            </div>
            {req.amountCents == null && (
              <>
                <label htmlFor="payAmount">Monto a pagar ({req.currency === 'CRC' ? '₡' : '$'})</label>
                <input
                  id="payAmount"
                  type="number"
                  min="0"
                  step="0.01"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  required
                />
              </>
            )}
            {error && <div className="error">{error}</div>}
            <button className="btn btn-red" onClick={pay} disabled={busy}>
              {busy ? 'Pagando…' : 'Pagar ahora'}
            </button>
            <div className="switch">
              <Link to="/">Volver a mi cuenta</Link>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
