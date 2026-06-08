import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from './auth'
import { AuthPage } from './pages/AuthPage'
import { Dashboard } from './pages/Dashboard'
import { PayRequest } from './pages/PayRequest'
import { ContributePool } from './pages/ContributePool'

export default function App() {
  const { user, loading } = useAuth()

  if (loading) {
    return <div className="center">Cargando…</div>
  }

  return (
    <Routes>
      <Route path="/login" element={user ? <Navigate to="/" replace /> : <AuthPage />} />
      <Route path="/" element={user ? <Dashboard /> : <Navigate to="/login" replace />} />
      {/* Share-link landing pages: gate to login, then render in place. */}
      <Route path="/cobro/:id" element={user ? <PayRequest /> : <AuthPage />} />
      <Route path="/vaquita/:id" element={user ? <ContributePool /> : <AuthPage />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
