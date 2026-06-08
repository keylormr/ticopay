import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, type Currency, type PaymentRequest } from '../api'
import { formatMoney } from '../format'
import { ShareCard } from '../components/ShareCard'

export function Cobros({ version, reload }: { version: number; reload: () => Promise<void> }) {
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
      setError(err instanceof ApiError ? err.message : 'No se pudo crear el cobro')
    } finally {
      setBusy(false)
    }
  }

  const shareUrl = createdId ? `${window.location.origin}/cobro/${createdId}` : ''

  return (
    <div className="grid">
      <div className="col">
        <section className="panel">
          <h2>Crear un cobro</h2>
          <p className="sub">Generá un enlace o QR para que te paguen. Compartilo por WhatsApp.</p>
          <form onSubmit={onCreate}>
            <label htmlFor="cto">Cobrar a (opcional, teléfono o correo)</label>
            <input
              id="cto"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              placeholder="Dejalo vacío para un cobro abierto"
            />
            <label htmlFor="ccur">Moneda</label>
            <select id="ccur" value={currency} onChange={(e) => setCurrency(e.target.value as Currency)}>
              <option value="CRC">Colones (₡)</option>
              <option value="USD">Dólares ($)</option>
            </select>
            <label htmlFor="camount">Monto (opcional)</label>
            <input
              id="camount"
              type="number"
              min="0"
              step="0.01"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder="Vacío = el pagador elige"
            />
            <label htmlFor="cdesc">Concepto</label>
            <input id="cdesc" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Entradas al concierto" />
            {error && <div className="error">{error}</div>}
            <button className="btn" type="submit" disabled={busy}>
              {busy ? 'Creando…' : 'Crear cobro'}
            </button>
          </form>
          {shareUrl && (
            <>
              <div className="ok" style={{ marginTop: 16 }}>¡Cobro creado! Compartí este enlace:</div>
              <ShareCard url={shareUrl} message="Te hago un cobro por Tico Pay:" />
            </>
          )}
        </section>
      </div>

      <div className="col">
        <section className="panel">
          <h2>Por cobrar / pagar</h2>
          <p className="sub">Cobros que te hicieron y los que creaste.</p>

          {incoming.length === 0 && outgoing.length === 0 && (
            <div className="empty">No tenés cobros todavía.</div>
          )}

          {incoming.map((r) => (
            <PayRow key={r.id} req={r} reload={async () => { await reload(); load() }} />
          ))}

          {outgoing.map((r) => (
            <div className="req-row" key={r.id}>
              <div className="tx-meta">
                <div className="name">Cobro a {r.requesterName}</div>
                <div className="desc">{r.description || 'Sin concepto'}</div>
              </div>
              <div className="req-right">
                <div className="req-amount">{r.amountCents != null ? formatMoney(r.amountCents, r.currency) : 'Abierto'}</div>
                <span className={`pill pill-${r.status}`}>{statusLabel(r.status)}</span>
              </div>
            </div>
          ))}
        </section>
      </div>
    </div>
  )
}

function PayRow({ req, reload }: { req: PaymentRequest; reload: () => Promise<void> }) {
  const [amount, setAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function pay() {
    setError('')
    const value = req.amountCents != null ? req.amountCents / 100 : Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError('Indicá un monto')
      return
    }
    setBusy(true)
    try {
      await api.payRequest(req.id, value)
      await reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'No se pudo pagar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="req-row">
      <div className="tx-meta">
        <div className="name">{req.requesterName} te cobra</div>
        <div className="desc">{req.description || 'Sin concepto'}</div>
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
            step="0.01"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder={req.currency === 'CRC' ? '₡' : '$'}
          />
        )}
        <button className="btn-pay" onClick={pay} disabled={busy}>
          {busy ? '…' : 'Pagar'}
        </button>
      </div>
    </div>
  )
}

function statusLabel(s: string): string {
  return s === 'paid' ? 'Pagado' : s === 'cancelled' ? 'Cancelado' : 'Pendiente'
}
