import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, tokens, type Account, type AuthResult, type Capabilities, type Currency, type User } from './api'

interface AuthState {
  user: User | null
  accounts: Account[]
  caps: Capabilities | null
  loading: boolean
  login: (email: string, password: string, totpCode?: string) => Promise<void>
  register: (input: { email: string; password: string; fullName: string; phone?: string }) => Promise<void>
  applyAuth: (res: AuthResult) => void
  logout: () => void
  refresh: () => Promise<void>
  setUser: (u: User) => void
  accountFor: (currency: Currency) => Account | undefined
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [accounts, setAccounts] = useState<Account[]>([])
  const [caps, setCaps] = useState<Capabilities | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!tokens.access) {
      setLoading(false)
      return
    }
    api
      .me()
      .then(({ user, accounts, capabilities }) => {
        setUser(user)
        setAccounts(accounts)
        setCaps(capabilities ?? null)
      })
      .catch(() => tokens.clear())
      .finally(() => setLoading(false))
  }, [])

  // loadCaps fetches the caller's back-office capabilities after a fresh login,
  // where the auth response carries no role. Non-blocking and best-effort: the
  // admin tab appears for staff once it resolves; regular users never need it.
  function loadCaps() {
    api
      .me()
      .then(({ capabilities }) => setCaps(capabilities ?? null))
      .catch(() => {})
  }

  async function login(email: string, password: string, totpCode?: string) {
    const res = await api.login(email, password, totpCode)
    tokens.set(res.accessToken, res.refreshToken)
    setUser(res.user)
    setAccounts(res.accounts)
    loadCaps()
  }

  async function register(input: { email: string; password: string; fullName: string; phone?: string }) {
    const res = await api.register(input)
    tokens.set(res.accessToken, res.refreshToken)
    setUser(res.user)
    setAccounts(res.accounts)
    loadCaps()
  }

  function applyAuth(res: AuthResult) {
    tokens.set(res.accessToken, res.refreshToken)
    setUser(res.user)
    setAccounts(res.accounts)
    loadCaps()
  }

  async function logout() {
    // Revoke all sessions server-side first (best-effort), then clear locally.
    // If the call fails (offline, expired token) we still log out on the client.
    try {
      await api.logout()
    } catch {
      // ignore — local logout proceeds regardless
    }
    tokens.clear()
    setUser(null)
    setAccounts([])
    setCaps(null)
  }

  async function refresh() {
    const { user, accounts, capabilities } = await api.me()
    setUser(user)
    setAccounts(accounts)
    setCaps(capabilities ?? null)
  }

  const value = useMemo<AuthState>(
    () => ({
      user,
      accounts,
      caps,
      loading,
      login,
      register,
      applyAuth,
      logout,
      refresh,
      setUser,
      accountFor: (currency) => accounts.find((a) => a.currency === currency),
    }),
    [user, accounts, caps, loading],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
