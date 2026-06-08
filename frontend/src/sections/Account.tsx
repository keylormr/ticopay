import { useState, type FormEvent } from 'react'
import { ApiError, api } from '../api'
import { useAuth } from '../auth'

const ID_TYPES = [
  { value: 'fisica', label: 'Cédula física (9 dígitos)' },
  { value: 'juridica', label: 'Cédula jurídica (10 dígitos)' },
  { value: 'dimex', label: 'DIMEX (extranjero, 11–12 dígitos)' },
]

export function Account() {
  const { user, setUser } = useAuth()
  const [idType, setIdType] = useState('fisica')
  const [idNumber, setIdNumber] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const verified = user?.kycStatus === 'verified'

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const res = await api.submitKyc({ idType, idNumber })
      if (user) setUser({ ...user, kycStatus: 'verified', idType: res.idType, idNumber: res.idNumber })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'No se pudo verificar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel narrow">
      <h2>Mi cuenta</h2>
      <p className="sub">
        {user?.fullName} · {user?.email}
        {user?.phone ? ` · ${user.phone}` : ''}
      </p>

      {verified ? (
        <div className="ok">
          ✓ Identidad verificada
          {user?.idNumber ? ` · ${user.idType} ${user.idNumber}` : ''}
        </div>
      ) : (
        <>
          <h2 style={{ marginTop: 18 }}>Verificá tu identidad</h2>
          <p className="sub">Requerido para mayores montos y cumplimiento (KYC). Validamos el formato tico.</p>
          <form onSubmit={onSubmit}>
            <label htmlFor="idType">Tipo de identificación</label>
            <select id="idType" value={idType} onChange={(e) => setIdType(e.target.value)}>
              {ID_TYPES.map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </select>
            <label htmlFor="idNumber">Número</label>
            <input
              id="idNumber"
              value={idNumber}
              onChange={(e) => setIdNumber(e.target.value)}
              placeholder="1-2345-6789"
              required
            />
            {error && <div className="error">{error}</div>}
            <button className="btn" type="submit" disabled={busy}>
              {busy ? 'Verificando…' : 'Verificar identidad'}
            </button>
          </form>
        </>
      )}
    </section>
  )
}
