import { useEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../api'
import { useAuth } from '../auth'
import { useI18n } from '../i18n'
import { Brand } from '../components/Brand'
import { LangToggle } from '../components/LangToggle'

export function VerifyEmail() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const { user, refresh } = useAuth()
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''
  const [state, setState] = useState<'pending' | 'ok' | 'error'>(token ? 'pending' : 'error')
  const ran = useRef(false)

  useEffect(() => {
    if (!token || ran.current) return
    ran.current = true // guard against React 18 StrictMode double-invoke
    api
      .verifyEmail(token)
      .then(async () => {
        setState('ok')
        if (user) await refresh().catch(() => {})
      })
      .catch(() => setState('error'))
  }, [token, user, refresh])

  return (
    <div className="auth-wrap">
      <div className="card">
        <div className="card-top">
          <Brand />
          <LangToggle />
        </div>
        <h1>{t('verify.title')}</h1>
        {state === 'pending' && <p className="sub">{t('verify.pending')}</p>}
        {state === 'ok' && <div className="ok">{t('verify.ok')}</div>}
        {state === 'error' && <div className="error">{t('verify.err')}</div>}
        <button className="btn" onClick={() => navigate(user ? '/' : '/login')}>
          {user ? t('verify.toApp') : t('reset.toLogin')}
        </button>
      </div>
    </div>
  )
}
