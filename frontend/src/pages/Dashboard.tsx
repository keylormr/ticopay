import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, type Transaction } from '../api'
import { useAuth } from '../auth'
import { Brand } from '../components/Brand'
import { formatCents, formatDate } from '../format'

export function Dashboard() {
  const { user, account, logout, refresh } = useAuth()
  const [txs, setTxs] = useState<Transaction[]>([])
  const [loadingTxs, setLoadingTxs] = useState(true)

  const [toEmail, setToEmail] = useState('')
  const [amount, setAmount] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const [busy, setBusy] = useState(false)

  async function loadTxs() {
    setLoadingTxs(true)
    try {
      const res = await api.transactions()
      setTxs(res.transactions)
    } catch {
      /* ignore — keep previous list */
    } finally {
      setLoadingTxs(false)
    }
  }

  useEffect(() => {
    loadTxs()
  }, [])

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
      await api.send({ toEmail, amount: value, description })
      setOk(`Enviaste ${formatCents(Math.round(value * 100))} a ${toEmail}`)
      setToEmail('')
      setAmount('')
      setDescription('')
      await Promise.all([refresh(), loadTxs()])
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'No se pudo enviar el pago')
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <header className="topbar">
        <Brand />
        <div className="who">
          <span>{user?.fullName}</span>
          <button className="btn-ghost" onClick={logout}>
            Salir
          </button>
        </div>
      </header>

      <main className="container">
        <section className="balance-card">
          <div className="label">Saldo disponible</div>
          <div className="amount">{account ? formatCents(account.balanceCents) : '—'}</div>
        </section>

        <div className="grid">
          <section className="panel">
            <h2>Enviar dinero</h2>
            <p className="sub">Transferí al instante a otro usuario por su correo.</p>
            <form onSubmit={onSend}>
              <label htmlFor="toEmail">Para (correo)</label>
              <input
                id="toEmail"
                type="email"
                value={toEmail}
                onChange={(e) => setToEmail(e.target.value)}
                placeholder="carlos@ticopay.cr"
                required
              />
              <label htmlFor="amount">Monto (₡)</label>
              <input
                id="amount"
                type="number"
                min="0"
                step="0.01"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder="5000"
                required
              />
              <label htmlFor="description">Detalle (opcional)</label>
              <input
                id="description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Almuerzo 🌮"
              />
              {error && <div className="error">{error}</div>}
              {ok && <div className="ok">{ok}</div>}
              <button className="btn btn-red" type="submit" disabled={busy}>
                {busy ? 'Enviando…' : 'Enviar pago'}
              </button>
            </form>
          </section>

          <section className="panel">
            <h2>Movimientos</h2>
            <p className="sub">Tus últimas transacciones.</p>
            {loadingTxs ? (
              <div className="empty">Cargando…</div>
            ) : txs.length === 0 ? (
              <div className="empty">Aún no tenés movimientos. ¡Enviá tu primer pago!</div>
            ) : (
              <ul className="tx-list">
                {txs.map((t) => (
                  <li className="tx-item" key={t.id}>
                    <div className={`tx-icon ${t.direction === 'in' ? 'tx-in' : 'tx-out'}`}>
                      {t.direction === 'in' ? '↓' : '↑'}
                    </div>
                    <div className="tx-meta">
                      <div className="name">{t.counterpart}</div>
                      <div className="desc">
                        {t.description || (t.direction === 'in' ? 'Pago recibido' : 'Pago enviado')} ·{' '}
                        {formatDate(t.createdAt)}
                      </div>
                    </div>
                    <div className={`tx-amount ${t.direction}`}>
                      {t.direction === 'in' ? '+' : '−'}
                      {formatCents(t.amountCents)}
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>
      </main>
    </>
  )
}
