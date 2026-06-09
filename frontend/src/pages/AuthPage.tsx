import { useState, type FormEvent } from 'react'
import { startAuthentication } from '@simplewebauthn/browser'
import { ApiError, api } from '../api'
import { useAuth } from '../auth'
import { Brand } from '../components/Brand'

export function AuthPage() {
  const { login, register, applyAuth } = useAuth()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [fullName, setFullName] = useState('')
  const [phone, setPhone] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      if (mode === 'login') {
        await login(email, password)
      } else {
        await register({ email, password, fullName, phone: phone || undefined })
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'No se pudo conectar con el servidor')
    } finally {
      setBusy(false)
    }
  }

  async function onPasskey() {
    setError('')
    const e = email.trim().toLowerCase()
    if (!e) {
      setError('Ingresá tu correo para usar la llave de acceso')
      return
    }
    setBusy(true)
    try {
      const begin = await api.passkeyLoginBegin(e)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const credential = await startAuthentication({ optionsJSON: begin.publicKey as any })
      const res = await api.passkeyLoginFinish({ sessionToken: begin.sessionToken, credential })
      applyAuth(res)
    } catch (err) {
      if (err instanceof ApiError) setError(err.message)
      else if (err instanceof Error && /abort|cancel|NotAllowed/i.test(err.name + err.message)) setError('Cancelaste el acceso con la llave')
      else setError('No se pudo usar la llave de acceso')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth-wrap">
      <form className="card" onSubmit={onSubmit}>
        <Brand />
        <h1>{mode === 'login' ? 'Iniciá sesión' : 'Creá tu cuenta'}</h1>
        <p className="sub">Pagos rápidos entre ticos, en colones. 🇨🇷</p>

        {mode === 'register' && (
          <>
            <label htmlFor="fullName">Nombre completo</label>
            <input
              id="fullName"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              placeholder="María Jiménez"
              required
            />
            <label htmlFor="phone">Teléfono (opcional)</label>
            <input
              id="phone"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              placeholder="8888-0000"
            />
          </>
        )}

        <label htmlFor="email">Correo electrónico</label>
        <input
          id="email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="vos@ejemplo.cr"
          required
        />

        <label htmlFor="password">Contraseña</label>
        <input
          id="password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder={mode === 'register' ? 'Mínimo 8 caracteres' : '••••••••'}
          required
        />

        {error && <div className="error">{error}</div>}

        <button className="btn" type="submit" disabled={busy}>
          {busy ? 'Procesando…' : mode === 'login' ? 'Entrar' : 'Registrarme'}
        </button>

        {mode === 'login' && (
          <>
            <div className="or-divider">o</div>
            <button type="button" className="btn btn-passkey" onClick={onPasskey} disabled={busy}>
              🔑 Entrar con llave de acceso
            </button>
          </>
        )}

        <div className="switch">
          {mode === 'login' ? '¿No tenés cuenta?' : '¿Ya tenés cuenta?'}{' '}
          <button
            type="button"
            onClick={() => {
              setMode(mode === 'login' ? 'register' : 'login')
              setError('')
            }}
          >
            {mode === 'login' ? 'Registrate' : 'Iniciá sesión'}
          </button>
        </div>

        {mode === 'login' && (
          <div className="hint">
            <strong>Cuenta demo:</strong> maria@ticopay.cr · contraseña <code>password123</code>
          </div>
        )}
      </form>
    </div>
  )
}
