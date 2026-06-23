import { useState, type FormEvent } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { ApiError, api } from '../api'
import { useI18n } from '../i18n'
import { Brand } from '../components/Brand'
import { LangToggle } from '../components/LangToggle'

export function ResetPassword() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (password.length < 8) {
      setError(t('reset.err.short'))
      return
    }
    if (password !== confirm) {
      setError(t('reset.err.match'))
      return
    }
    setBusy(true)
    try {
      await api.resetPassword(token, password)
      setDone(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('auth.err.connect'))
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
        <h1>{t('reset.title')}</h1>

        {!token ? (
          <div className="error">{t('reset.err.noToken')}</div>
        ) : done ? (
          <>
            <div className="ok">{t('reset.done')}</div>
            <button className="btn" onClick={() => navigate('/login')}>
              {t('reset.toLogin')}
            </button>
          </>
        ) : (
          <form onSubmit={onSubmit}>
            <label htmlFor="newPassword">{t('reset.new')}</label>
            <input
              id="newPassword"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={t('auth.password.ph.register')}
              required
            />
            <label htmlFor="confirmPassword">{t('reset.confirm')}</label>
            <input
              id="confirmPassword"
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              placeholder="••••••••"
              required
            />
            {error && <div className="error">{error}</div>}
            <button className="btn" type="submit" disabled={busy}>
              {busy ? t('auth.processing') : t('reset.btn')}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
