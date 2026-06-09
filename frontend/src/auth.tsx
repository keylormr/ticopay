import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, tokens, type Account, type AuthResult, type Currency, type User } from './api'

interface AuthState {
  user: User | null
  accounts: Account[]
  loading: boolean
  login: (email: string, password: string) => Promise<void>
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
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!tokens.access) {
      setLoading(false)
      return
    }
    api
      .me()
      .then(({ user, accounts }) => {
        setUser(user)
        setAccounts(accounts)
      })
      .catch(() => tokens.clear())
      .finally(() => setLoading(false))
  }, [])

  async function login(email: string, password: string) {
    const res = await api.login(email, password)
    tokens.set(res.accessToken, res.refreshToken)
    setUser(res.user)
    setAccounts(res.accounts)
  }

  async function register(input: { email: string; password: string; fullName: string; phone?: string }) {
    const res = await api.register(input)
    tokens.set(res.accessToken, res.refreshToken)
    setUser(res.user)
    setAccounts(res.accounts)
  }

  function applyAuth(res: AuthResult) {
    tokens.set(res.accessToken, res.refreshToken)
    setUser(res.user)
    setAccounts(res.accounts)
  }

  function logout() {
    tokens.clear()
    setUser(null)
    setAccounts([])
  }

  async function refresh() {
    const { user, accounts } = await api.me()
    setUser(user)
    setAccounts(accounts)
  }

  const value = useMemo<AuthState>(
    () => ({
      user,
      accounts,
      loading,
      login,
      register,
      applyAuth,
      logout,
      refresh,
      setUser,
      accountFor: (currency) => accounts.find((a) => a.currency === currency),
    }),
    [user, accounts, loading],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
