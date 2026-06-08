const API_URL = (import.meta.env.VITE_API_URL ?? 'http://localhost:8080').replace(/\/$/, '')

export type Currency = 'CRC' | 'USD' | 'BTC' | 'ETH' | 'USDT'

export interface Rates {
  crc: ExchangeRate
  crypto: Record<string, number>
  usdPerUnit: Record<string, number>
  updatedAt: string
}

export interface User {
  id: string
  email: string
  phone?: string
  fullName: string
  kycStatus: 'none' | 'verified'
  idType?: string
  idNumber?: string
  createdAt: string
}

export interface Account {
  id: string
  currency: Currency
  balanceCents: number
}

export interface Transaction {
  id: string
  direction: 'in' | 'out' | 'self'
  counterpart: string
  amountCents: number
  currency: Currency
  description: string
  status: string
  kind: 'transfer' | 'conversion' | 'request' | 'pool'
  createdAt: string
}

export interface ExchangeRate {
  buy: number
  sell: number
  date: string
  source: string
}

export interface PaymentRequest {
  id: string
  requesterName: string
  amountCents: number | null
  currency: Currency
  description: string
  status: 'pending' | 'paid' | 'cancelled'
  direction?: 'incoming' | 'outgoing'
  createdAt: string
}

export interface Pool {
  id: string
  ownerName: string
  isOwner: boolean
  name: string
  description: string
  goalCents: number
  raisedCents: number
  currency: Currency
  status: 'open' | 'closed'
  createdAt: string
}

export interface PoolContribution {
  name: string
  amountCents: number
  createdAt: string
}

export interface AuthResult {
  accessToken: string
  refreshToken: string
  user: User
  accounts: Account[]
}

const ACCESS_KEY = 'ticopay.access'
const REFRESH_KEY = 'ticopay.refresh'

export const tokens = {
  get access() {
    return localStorage.getItem(ACCESS_KEY)
  },
  get refresh() {
    return localStorage.getItem(REFRESH_KEY)
  },
  set(access: string, refresh: string) {
    localStorage.setItem(ACCESS_KEY, access)
    localStorage.setItem(REFRESH_KEY, refresh)
  },
  clear() {
    localStorage.removeItem(ACCESS_KEY)
    localStorage.removeItem(REFRESH_KEY)
  },
}

class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function parse<T>(res: Response): Promise<T> {
  const text = await res.text()
  const body = text ? JSON.parse(text) : {}
  if (!res.ok) {
    throw new ApiError(res.status, body.error ?? `Error ${res.status}`)
  }
  return body as T
}

async function refreshTokens(): Promise<boolean> {
  const refresh = tokens.refresh
  if (!refresh) return false
  const res = await fetch(`${API_URL}/api/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refreshToken: refresh }),
  })
  if (!res.ok) return false
  const body = (await res.json()) as { accessToken: string; refreshToken: string }
  tokens.set(body.accessToken, body.refreshToken)
  return true
}

async function request<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Content-Type', 'application/json')
  if (tokens.access) headers.set('Authorization', `Bearer ${tokens.access}`)

  const res = await fetch(`${API_URL}${path}`, { ...init, headers })
  if (res.status === 401 && retry && (await refreshTokens())) {
    return request<T>(path, init, false)
  }
  return parse<T>(res)
}

const body = (v: unknown) => JSON.stringify(v)

export const api = {
  // --- auth ---
  register(input: { email: string; password: string; fullName: string; phone?: string }) {
    return request<AuthResult>('/api/auth/register', { method: 'POST', body: body(input) }, false)
  },
  login(email: string, password: string) {
    return request<AuthResult>('/api/auth/login', { method: 'POST', body: body({ email, password }) }, false)
  },
  me() {
    return request<{ user: User; accounts: Account[] }>('/api/me')
  },

  // --- money ---
  transactions() {
    return request<{ transactions: Transaction[] }>('/api/transactions')
  },
  send(input: { to: string; amount: number; currency: Currency; description: string }) {
    return request<{ id: string; newBalance: number }>('/api/transactions', { method: 'POST', body: body(input) })
  },
  convert(input: { from: Currency; to: Currency; amount: number }) {
    return request<{ fromCents: number; toCents: number; rate: ExchangeRate }>('/api/convert', {
      method: 'POST',
      body: body(input),
    })
  },
  exchangeRate() {
    return request<ExchangeRate>('/api/exchange-rate')
  },
  rates() {
    return request<Rates>('/api/rates')
  },

  // --- KYC ---
  submitKyc(input: { idType: string; idNumber: string }) {
    return request<{ kycStatus: string; idType: string; idNumber: string }>('/api/kyc', {
      method: 'POST',
      body: body(input),
    })
  },

  // --- payment requests (cobros) ---
  createRequest(input: { to?: string; amount?: number; currency: Currency; description: string }) {
    return request<{ id: string; currency: Currency }>('/api/requests', { method: 'POST', body: body(input) })
  },
  listRequests() {
    return request<{ incoming: PaymentRequest[]; outgoing: PaymentRequest[] }>('/api/requests')
  },
  getRequest(id: string) {
    return request<PaymentRequest>(`/api/requests/${id}`)
  },
  payRequest(id: string, amount?: number) {
    return request<{ status: string; amountCents: number; currency: Currency }>(`/api/requests/${id}/pay`, {
      method: 'POST',
      body: body({ amount: amount ?? 0 }),
    })
  },

  // --- vaquitas (pools) ---
  createPool(input: { name: string; description: string; goalAmount: number; currency: Currency }) {
    return request<{ id: string }>('/api/pools', { method: 'POST', body: body(input) })
  },
  listPools() {
    return request<{ mine: Pool[]; joined: Pool[] }>('/api/pools')
  },
  getPool(id: string) {
    return request<{ pool: Pool; contributions: PoolContribution[] }>(`/api/pools/${id}`)
  },
  contributePool(id: string, amount: number) {
    return request<{ status: string; amountCents: number }>(`/api/pools/${id}/contribute`, {
      method: 'POST',
      body: body({ amount }),
    })
  },
}

export { ApiError }
