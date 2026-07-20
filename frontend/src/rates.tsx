import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { api, tokens, type Rates } from './api'
import { useAuth } from './auth'

interface RatesState {
  rates: Rates | null
  refresh: () => void
}

const RatesContext = createContext<RatesState | null>(null)

// RatesProvider loads the exchange-rate feed once and shares it across the app,
// replacing the separate fetches the dashboard and the convert screen used to
// each make. On a persisted session it fires in parallel with the auth me()
// call (both start from the token already in storage), so rates and balances
// arrive together instead of in a waterfall — and the convert preview is
// instant because the rates are already in memory.
export function RatesProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth()
  const [rates, setRates] = useState<Rates | null>(null)

  const refresh = useCallback(() => {
    api
      .rates()
      .then(setRates)
      .catch(() => {})
  }, [])

  // Persisted session: a token already exists at mount, so load immediately
  // (in parallel with me()), not after the user object resolves.
  useEffect(() => {
    if (tokens.access) refresh()
  }, [refresh])

  // Fresh login (no token at mount): load once the user appears.
  useEffect(() => {
    if (user && !rates) refresh()
  }, [user, rates, refresh])

  return <RatesContext.Provider value={{ rates, refresh }}>{children}</RatesContext.Provider>
}

export function useRates(): RatesState {
  const ctx = useContext(RatesContext)
  if (!ctx) throw new Error('useRates must be used within RatesProvider')
  return ctx
}
