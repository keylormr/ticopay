import { useState, type FormEvent } from 'react'
import { ApiError, api, type Currency } from '../api'
import { formatMoney } from '../format'

export function SendMoney({ reload }: { reload: () => Promise<void> }) {
  const [to, setTo] = useState('')
  const [currency, setCurrency] = useState<Currency>('CRC')
  const [amount, setAmount] = useState('')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')

  async function onSend(e: FormEvent) {
    e.preventDefault()
    setError('')
    setOk('')
    const value = Number(amount)
    if (!Number.isFinite(value) || value <= 0) {
      setError('Ingresá un monto válido')
      return
    }
    setBusy(true)
    try {
      await api.send({ to: to.trim(), amount: value, currency, description })
      setOk(`Enviaste ${formatMoney(Math.round(value * 100), currency)} a ${to.trim()}`)
      setTo('')
      setAmount('')
      setDescription('')
      await reload()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'No se pudo enviar el pago')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel narrow">
      <h2>Enviar dinero</h2>
      <p className="sub">Al instante, por número de teléfono o correo.</p>
      <form onSubmit={onSend}>
        <label htmlFor="to">Para (teléfono o correo)</label>
        <input
          id="to"
          value={to}
          onChange={(e) => setTo(e.target.value)}
          placeholder="8888-0000 o carlos@ticopay.cr"
          required
        />
        <label htmlFor="currency">Moneda</label>
        <select id="currency" value={currency} onChange={(e) => setCurrency(e.target.value as Currency)}>
          <option value="CRC">Colones (₡)</option>
          <option value="USD">Dólares ($)</option>
        </select>
        <label htmlFor="amount">Monto ({currency === 'CRC' ? '₡' : '$'})</label>
        <input
          id="amount"
          type="number"
          min="0"
          step="0.01"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          placeholder={currency === 'CRC' ? '5000' : '20'}
          required
        />
        <label htmlFor="desc">Detalle (opcional)</label>
        <input id="desc" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Almuerzo 🌮" />
        {error && <div className="error">{error}</div>}
        {ok && <div className="ok">{ok}</div>}
        <button className="btn btn-red" type="submit" disabled={busy}>
          {busy ? 'Enviando…' : 'Enviar pago'}
        </button>
      </form>
    </section>
  )
}
