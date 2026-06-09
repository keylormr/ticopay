import { useEffect, useState, type FormEvent } from 'react'
import { startRegistration } from '@simplewebauthn/browser'
import { ApiError, api } from '../api'
import { useAuth } from '../auth'
import { formatDate } from '../format'

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
    <>
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

      <Passkeys />
    </>
  )
}

function Passkeys() {
  const [list, setList] = useState<{ id: string; name: string; createdAt: string }[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')

  function load() {
    api
      .listPasskeys()
      .then((r) => setList(r.passkeys))
      .catch(() => {})
  }
  useEffect(load, [])

  async function add() {
    setError('')
    setOk('')
    setBusy(true)
    try {
      const begin = await api.passkeyRegisterBegin()
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const credential = await startRegistration({ optionsJSON: begin.publicKey as any })
      await api.passkeyRegisterFinish({ sessionToken: begin.sessionToken, credential, name: 'Mi dispositivo' })
      setOk('¡Llave de acceso agregada! Ya podés entrar sin contraseña.')
      load()
    } catch (err) {
      if (err instanceof ApiError) setError(err.message)
      else if (err instanceof Error && /abort|cancel|NotAllowed/i.test(err.name + err.message))
        setError('Cancelaste el registro de la llave')
      else setError('No se pudo agregar la llave de acceso')
    } finally {
      setBusy(false)
    }
  }

  async function remove(id: string) {
    try {
      await api.deletePasskey(id)
      load()
    } catch {
      /* ignore */
    }
  }

  return (
    <section className="panel narrow" style={{ marginTop: 18 }}>
      <h2>🔑 Llaves de acceso</h2>
      <p className="sub">Entrá sin contraseña con Face ID, huella o el PIN de tu dispositivo. Más seguro que una clave.</p>

      {list.length === 0 ? (
        <div className="empty">Todavía no agregaste ninguna llave.</div>
      ) : (
        <ul className="tx-list">
          {list.map((p) => (
            <li className="tx-item" key={p.id}>
              <div className="tx-icon tx-in">🔑</div>
              <div className="tx-meta">
                <div className="name">{p.name}</div>
                <div className="desc">Agregada el {formatDate(p.createdAt)}</div>
              </div>
              <button className="btn-ghost" onClick={() => remove(p.id)}>
                Eliminar
              </button>
            </li>
          ))}
        </ul>
      )}

      {error && <div className="error">{error}</div>}
      {ok && <div className="ok">{ok}</div>}
      <button className="btn" onClick={add} disabled={busy}>
        {busy ? 'Esperando tu dispositivo…' : 'Agregar llave de acceso'}
      </button>
    </section>
  )
}
