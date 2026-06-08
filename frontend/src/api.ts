const API_URL = (import.meta.env.VITE_API_URL ?? 'http://localhost:8080').replace(/\/$/, '')

export interface User {
  id: string
  email: string
  phone?: string
  fullName: string
  createdAt: string
}

export interface Account {
  id: string
  currency: string
  balanceCents: number
}

export interface Transaction {
  id: string
  direction: 'in' | 'out'
  counterpart: string
  amountCents: number
  currency: string
  description: string
  status: string
  createdAt: string
}

export interface AuthResult {
  accessToken: string
  refreshToken: string
  user: User
  account: Account
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

export const api = {
  async register(input: { email: string; password: string; fullName: string; phone?: string }): Promise<AuthResult> {
    return request<AuthResult>('/api/auth/register', { method: 'POST', body: JSON.stringify(input) }, false)
  },
  async login(email: string, password: string): Promise<AuthResult> {
    return request<AuthResult>('/api/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }, false)
  },
  async me(): Promise<{ user: User; account: Account }> {
    return request('/api/me')
  },
  async transactions(): Promise<{ transactions: Transaction[] }> {
    return request('/api/transactions')
  },
  async send(input: { toEmail: string; amount: number; description: string }) {
    return request('/api/transactions', { method: 'POST', body: JSON.stringify(input) })
  },
}

export { ApiError }
