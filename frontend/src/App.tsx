import { lazy, Suspense, useEffect, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from './auth'
import { useI18n } from './i18n'
import { AuthPage } from './pages/AuthPage'

// The dashboard and the email/share landing pages are code-split: only the
// screen actually visited is downloaded. AuthPage stays eager — it's the first
// thing a logged-out visitor sees, so it must render without a second fetch.
const Dashboard = lazy(() => import('./pages/Dashboard').then((m) => ({ default: m.Dashboard })))
const PayRequest = lazy(() => import('./pages/PayRequest').then((m) => ({ default: m.PayRequest })))
const ContributePool = lazy(() => import('./pages/ContributePool').then((m) => ({ default: m.ContributePool })))
const ResetPassword = lazy(() => import('./pages/ResetPassword').then((m) => ({ default: m.ResetPassword })))
const VerifyEmail = lazy(() => import('./pages/VerifyEmail').then((m) => ({ default: m.VerifyEmail })))

// Booting shows a spinner and, once the wait crosses ~3s, explains that the
// (Render Free) backend is waking up — turning a seemingly frozen "Loading…"
// into an understandable state instead of a first impression of a hung app.
function Booting() {
  const { t } = useI18n()
  const [slow, setSlow] = useState(false)
  useEffect(() => {
    const id = setTimeout(() => setSlow(true), 3000)
    return () => clearTimeout(id)
  }, [])
  return (
    <div className="boot" role="status" aria-live="polite">
      <div className="boot-spinner" aria-hidden="true" />
      <p className="boot-msg">{slow ? t('common.waking') : t('common.loading')}</p>
    </div>
  )
}

export default function App() {
  const { user, loading } = useAuth()

  if (loading) {
    return <Booting />
  }

  return (
    <Suspense fallback={<Booting />}>
      <Routes>
        <Route path="/login" element={user ? <Navigate to="/" replace /> : <AuthPage />} />
        {/* Email-link landing pages — reachable with or without a session. */}
        <Route path="/reset" element={<ResetPassword />} />
        <Route path="/verify-email" element={<VerifyEmail />} />
        <Route path="/" element={user ? <Dashboard /> : <Navigate to="/login" replace />} />
        {/* Share-link landing pages: gate to login, then render in place. */}
        <Route path="/cobro/:id" element={user ? <PayRequest /> : <AuthPage />} />
        <Route path="/vaquita/:id" element={user ? <ContributePool /> : <AuthPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
  )
}
