import { useState, type FormEvent } from 'react'
import { startAuthentication } from '@simplewebauthn/browser'
import { ApiError, api } from '../api'
import { useAuth } from '../auth'
import { useI18n } from '../i18n'
import { Brand } from '../components/Brand'
import { LangToggle } from '../components/LangToggle'

export function AuthPage() {
  const { login, register, applyAuth } = useAuth()
  const { t } = useI18n()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [fullName, setFullName] = useState('')
  const [phone, setPhone] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [showRecovery, setShowRecovery] = useState(false)
  const [recoveryCode, setRecoveryCode] = useState('')
  const [totpRequired, setTotpRequired] = useState(false)
  const [totpCode, setTotpCode] = useState('')
  const [showForgot, setShowForgot] = useState(false)
  const [forgotSent, setForgotSent] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    // While the forgot-password box is open, the form's submit (incl. Enter)
    // must send the reset link, not attempt a login.
    if (mode === 'login' && showForgot) {
      if (!forgotSent) await onForgot(e)
      return
    }
    setError('')
    setBusy(true)
    try {
      if (mode === 'login') {
        await login(email, password, totpCode || undefined)
      } else {
        await register({ email, password, fullName, phone: phone || undefined })
      }
    } catch (err) {
      // 428: the account has 2FA on and the server wants a code.
      if (err instanceof ApiError && err.status === 428) {
        setTotpRequired(true)
      } else {
        setError(err instanceof ApiError ? err.message : t('auth.err.connect'))
      }
    } finally {
      setBusy(false)
    }
  }

  async function onPasskey() {
    setError('')
    const e = email.trim().toLowerCase()
    if (!e) {
      setError(t('auth.err.passkeyEmail'))
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
      else if (err instanceof Error && /abort|cancel|NotAllowed/i.test(err.name + err.message)) setError(t('auth.err.passkeyCancel'))
      else setError(t('auth.err.passkey'))
    } finally {
      setBusy(false)
    }
  }

  async function onForgot(e: FormEvent) {
    e.preventDefault()
    setError('')
    const addr = email.trim().toLowerCase()
    if (!addr) {
      setError(t('auth.err.passkeyEmail'))
      return
    }
    setBusy(true)
    try {
      await api.forgotPassword(addr)
      setForgotSent(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('auth.err.connect'))
    } finally {
      setBusy(false)
    }
  }

  async function onRecovery() {
    setError('')
    const e = email.trim().toLowerCase()
    if (!e) {
      setError(t('auth.err.passkeyEmail'))
      return
    }
    if (!recoveryCode.trim()) {
      setError(t('auth.err.recoveryCode'))
      return
    }
    setBusy(true)
    try {
      const res = await api.recoveryLogin(e, recoveryCode)
      applyAuth(res)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('auth.err.connect'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth-wrap">
      <form className="card" onSubmit={onSubmit}>
        <div className="card-top">
          <Brand />
          <LangToggle />
        </div>
        <h1>{mode === 'login' ? t('auth.login.title') : t('auth.register.title')}</h1>
        <p className="sub">{t('app.tagline')}</p>

        {mode === 'register' && (
          <>
            <label htmlFor="fullName">{t('auth.fullName')}</label>
            <input id="fullName" value={fullName} onChange={(e) => setFullName(e.target.value)} placeholder="María Jiménez" required />
            <label htmlFor="phone">{t('auth.phone')}</label>
            <input id="phone" value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="8888-0000" />
          </>
        )}

        <label htmlFor="email">{t('auth.email')}</label>
        <input
          id="email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder={t('auth.email.ph')}
          required
        />

        <label htmlFor="password">{t('auth.password')}</label>
        <input
          id="password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder={mode === 'register' ? t('auth.password.ph.register') : '••••••••'}
          required
        />

        {mode === 'login' && !showForgot && (
          <button type="button" className="link-btn link-right" onClick={() => setShowForgot(true)}>
            {t('auth.forgot.link')}
          </button>
        )}

        {mode === 'login' && showForgot && (
          <div className="forgot-box">
            {forgotSent ? (
              <div className="ok">{t('auth.forgot.sent')}</div>
            ) : (
              <>
                <p className="sub">{t('auth.forgot.hint')}</p>
                <button type="button" className="btn" onClick={onForgot} disabled={busy}>
                  {busy ? t('auth.processing') : t('auth.forgot.btn')}
                </button>
              </>
            )}
            <button
              type="button"
              className="link-btn"
              onClick={() => {
                setShowForgot(false)
                setForgotSent(false)
                setError('')
              }}
            >
              {t('auth.forgot.back')}
            </button>
          </div>
        )}

        {totpRequired && mode === 'login' && (
          <>
            <label htmlFor="totpCode">{t('auth.totp.label')}</label>
            <input
              id="totpCode"
              inputMode="numeric"
              autoComplete="one-time-code"
              value={totpCode}
              onChange={(e) => setTotpCode(e.target.value)}
              placeholder="123 456"
              autoFocus
            />
          </>
        )}

        {error && <div className="error">{error}</div>}

        {!(mode === 'login' && showForgot) && (
          <button className="btn" type="submit" disabled={busy}>
            {busy ? t('auth.processing') : mode === 'login' ? t('auth.btn.login') : t('auth.btn.register')}
          </button>
        )}

        {mode === 'login' && !showForgot && (
          <>
            <div className="or-divider">{t('auth.or')}</div>
            <button type="button" className="btn btn-passkey" onClick={onPasskey} disabled={busy}>
              {t('auth.passkey')}
            </button>

            {showRecovery ? (
              <>
                <label htmlFor="recoveryCode" style={{ marginTop: 12 }}>
                  {t('auth.recovery.label')}
                </label>
                <input
                  id="recoveryCode"
                  value={recoveryCode}
                  onChange={(e) => setRecoveryCode(e.target.value)}
                  placeholder="ABCD-EF23"
                  autoComplete="off"
                />
                <button type="button" className="btn" onClick={onRecovery} disabled={busy}>
                  {busy ? t('auth.processing') : t('auth.recovery.btn')}
                </button>
              </>
            ) : (
              <button type="button" className="link-btn" onClick={() => setShowRecovery(true)}>
                {t('auth.recovery.link')}
              </button>
            )}
          </>
        )}

        <div className="switch">
          {mode === 'login' ? t('auth.noAccount') : t('auth.haveAccount')}{' '}
          <button
            type="button"
            onClick={() => {
              setMode(mode === 'login' ? 'register' : 'login')
              setError('')
              setTotpRequired(false)
              setTotpCode('')
              setShowForgot(false)
              setForgotSent(false)
            }}
          >
            {mode === 'login' ? t('auth.switch.register') : t('auth.switch.login')}
          </button>
        </div>

        {mode === 'login' && (
          <div className="hint">
            <strong>{t('auth.demo')}</strong> maria@ticopay.cr · {t('auth.demo.pwd')} <code>password123</code>
          </div>
        )}
      </form>
    </div>
  )
}
