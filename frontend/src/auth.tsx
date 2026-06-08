import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, tokens, type Account, type User } from './api'

interface AuthState {
  user: User | null
  account: Account | null
  loading: boolean
  login: (email: string, password: string) => Promise<void>
  register: (input: { email: string; password: string; fullName: string; phone?: string }) => Promise<void>
  logout: () => void
  refresh: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [account, setAccount] = useState<Account | null>(null)
  const [loading, setLoading] = useState(true)

  // Restore session on first load if we have a stored token.
  useEffect(() => {
    if (!tokens.access) {
      setLoading(false)
      return
    }
    api
      .me()
      .then(({ user, account }) => {
        setUser(user)
        setAccount(account)
      })
      .catch(() => tokens.clear())
      .finally(() => setLoading(false))
  }, [])

  async function login(email: string, password: string) {
    const res = await api.login(email, password)
    tokens.set(res.accessToken, res.refreshToken)
    setUser(res.user)
    setAccount(res.account)
  }

  async function register(input: { email: string; password: string; fullName: string; phone?: string }) {
    const res = await api.register(input)
    tokens.set(res.accessToken, res.refreshToken)
    setUser(res.user)
    setAccount(res.account)
  }

  function logout() {
    tokens.clear()
    setUser(null)
    setAccount(null)
  }

  async function refresh() {
    const { user, account } = await api.me()
    setUser(user)
    setAccount(account)
  }

  const value = useMemo<AuthState>(
    () => ({ user, account, loading, login, register, logout, refresh }),
    [user, account, loading],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
