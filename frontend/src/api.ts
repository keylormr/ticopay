const API_URL = (import.meta.env.VITE_API_URL ?? 'http://localhost:8080').replace(/\/$/, '')

// warmBackend fires a cheap request so a cold backend (Render Free sleeps after
// ~15 min idle) starts waking while the user is still reading the login screen,
// hiding much of the ~50s cold start. It also opens the TCP/TLS connection early
// so the first real request is faster. Best-effort; failures are ignored.
export function warmBackend(): void {
  void fetch(`${API_URL}/health`).catch(() => {})
}

export type Currency =
  | 'CRC'
  | 'USD'
  | 'EUR'
  | 'MXN'
  | 'BTC'
  | 'ETH'
  | 'USDT'
  | 'USDC'
  | 'BNB'
  | 'SOL'
  | 'XRP'
  | 'ADA'
  | 'DOGE'
  | 'TRX'
  | 'DOT'
  | 'LTC'
  | 'LINK'
  | 'AVAX'
  | 'MATIC'

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
  emailVerified: boolean
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
  kind: 'transfer' | 'conversion' | 'request' | 'pool' | 'service' | 'sinpe'
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

export interface Biller {
  id: string
  name: string
  category: string
  icon: string
  refLabel: string
  refPlaceholder: string
}

export interface Merchant {
  id: string
  name: string
  category: string
  legalName?: string
  idType?: string
  idNumber?: string
  status: 'pending' | 'verified' | 'rejected'
  rejectReason?: string
  commissionBps: number
  ownerEmail?: string
  createdAt: string
}

export interface Capabilities {
  backoffice: boolean
  reports: boolean
  manageUsers: boolean
  verifyMerchants: boolean
  setCommission: boolean
}

export interface AdminUser {
  id: string
  email: string
  fullName: string
  phone?: string
  role: string
  kycStatus: string
  emailVerified: boolean
  disabled: boolean
  createdAt: string
}

export interface ReportOverview {
  users: { total: number; byRole: Record<string, number>; active30d: number; disabled: number }
  merchants: Record<string, number>
  transactions: { total: number; byKind: Record<string, number> }
  volumeByCurrency: { currency: Currency; amountCents: number; count: number }[]
  feesByCurrency: { currency: Currency; amountCents: number }[]
}

export interface SeriesPoint {
  date: string
  value: number
}

export interface ByKindRow {
  kind: string
  count: number
  amountCents: number
}

export interface LedgerHealth {
  balanced: boolean
  netByCurrency: { currency: Currency; netCents: number }[]
  systemAccounts: { account: string; currency: Currency; balanceCents: number }[]
}

export interface TxReportRow {
  id: string
  createdAt: string
  kind: string
  status: string
  currency: Currency
  amountCents: number
  feeCents: number
  fromEmail: string
  toEmail: string
  description: string
}

export interface AuditEntry {
  id: number
  action: string
  target: string
  detail: Record<string, unknown>
  createdAt: string
  actorEmail: string
  actorName: string
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
  headers.set('X-Lang', localStorage.getItem('ticopay.lang') || 'es')
  if (tokens.access) headers.set('Authorization', `Bearer ${tokens.access}`)

  const res = await fetch(`${API_URL}${path}`, { ...init, headers })
  if (res.status === 401 && retry && (await refreshTokens())) {
    // Only replay requests that are safe to repeat: GET/HEAD are idempotent,
    // and money POSTs carry an Idempotency-Key the server dedupes on. A plain
    // money POST is never auto-retried, so a stale-token failure can't double
    // it; the caller surfaces the error and the user retries deliberately.
    const method = (init.method ?? 'GET').toUpperCase()
    if (method === 'GET' || method === 'HEAD' || headers.has('Idempotency-Key')) {
      return request<T>(path, init, false)
    }
  }
  return parse<T>(res)
}

const body = (v: unknown) => JSON.stringify(v)

// idem returns request init carrying a fresh Idempotency-Key so the server
// dedupes a retried call (network re-send) without moving money twice. A new
// key per call means two deliberate clicks remain two distinct operations.
const idem = (): RequestInit => ({ headers: { 'Idempotency-Key': crypto.randomUUID() } })

export const api = {
  // --- auth ---
  register(input: { email: string; password: string; fullName: string; phone?: string }) {
    return request<AuthResult>('/api/auth/register', { method: 'POST', body: body(input) }, false)
  },
  login(email: string, password: string, totpCode?: string) {
    return request<AuthResult>('/api/auth/login', { method: 'POST', body: body({ email, password, totpCode }) }, false)
  },
  me() {
    return request<{ user: User; accounts: Account[]; capabilities?: Capabilities }>('/api/me')
  },
  logout() {
    // Revokes every session server-side (bumps token_version). Best-effort:
    // the client clears its local tokens regardless of the response.
    return request<{ status: string }>('/api/auth/logout', { method: 'POST', body: '{}' }, false)
  },
  forgotPassword(email: string) {
    return request<{ status: string }>('/api/auth/forgot', { method: 'POST', body: body({ email }) }, false)
  },
  resetPassword(token: string, password: string) {
    return request<{ status: string }>('/api/auth/reset', { method: 'POST', body: body({ token, password }) }, false)
  },
  verifyEmail(token: string) {
    return request<{ status: string }>('/api/auth/verify-email', { method: 'POST', body: body({ token }) }, false)
  },
  resendVerification() {
    return request<{ status: string }>('/api/auth/verify-email/send', { method: 'POST', body: '{}' })
  },

  // --- money ---
  transactions() {
    return request<{ transactions: Transaction[] }>('/api/transactions')
  },
  send(input: { to: string; amount: number; currency: Currency; description: string }) {
    return request<{ id: string; newBalance: number }>('/api/transactions', { method: 'POST', body: body(input), ...idem() })
  },
  sinpe(input: { toPhone: string; amount: number; description: string }) {
    return request<{ comprobante: string; recipientName: string; amountCents: number; at: string; simulated: boolean }>('/api/sinpe', {
      method: 'POST',
      body: body(input),
      ...idem(),
    })
  },
  convert(input: { from: Currency; to: Currency; amount: number }) {
    return request<{ fromCents: number; toCents: number; rate: ExchangeRate }>('/api/convert', {
      method: 'POST',
      body: body(input),
      ...idem(),
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
    return request<{ status: string; amountCents: number; currency: Currency; feeCents: number; netCents: number }>(
      `/api/requests/${id}/pay`,
      { method: 'POST', body: body({ amount: amount ?? 0 }) },
    )
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
      ...idem(),
    })
  },

  // --- passkeys (WebAuthn) ---
  passkeyRegisterBegin() {
    return request<{ publicKey: unknown; sessionToken: string }>('/api/passkeys/register/begin', { method: 'POST', body: '{}' })
  },
  passkeyRegisterFinish(input: { sessionToken: string; credential: unknown; name: string }) {
    return request<{ status: string }>('/api/passkeys/register/finish', { method: 'POST', body: body(input) })
  },
  passkeyLoginBegin(email: string) {
    return request<{ publicKey: unknown; sessionToken: string }>('/api/auth/passkey/begin', {
      method: 'POST',
      body: body({ email }),
    })
  },
  passkeyLoginFinish(input: { sessionToken: string; credential: unknown }) {
    return request<AuthResult>('/api/auth/passkey/finish', { method: 'POST', body: body(input) }, false)
  },
  listPasskeys() {
    return request<{ passkeys: { id: string; name: string; createdAt: string }[] }>('/api/passkeys')
  },
  deletePasskey(id: string) {
    return request<{ status: string }>(`/api/passkeys/${id}`, { method: 'DELETE' })
  },

  // --- recovery codes (passkey fallback) ---
  recoveryStatus() {
    return request<{ remaining: number }>('/api/passkeys/recovery-codes')
  },
  generateRecoveryCodes() {
    return request<{ codes: string[] }>('/api/passkeys/recovery-codes', { method: 'POST', body: '{}' })
  },
  recoveryLogin(email: string, code: string) {
    return request<AuthResult>('/api/auth/recovery', { method: 'POST', body: body({ email, code }) }, false)
  },

  // --- TOTP 2FA (authenticator app) ---
  totpStatus() {
    return request<{ enabled: boolean }>('/api/totp')
  },
  totpSetup() {
    return request<{ secret: string; otpauthUrl: string }>('/api/totp/setup', { method: 'POST', body: '{}' })
  },
  totpConfirm(code: string) {
    return request<{ status: string }>('/api/totp/confirm', { method: 'POST', body: body({ code }) })
  },
  totpDisable(code: string) {
    return request<{ status: string }>('/api/totp/disable', { method: 'POST', body: body({ code }) })
  },

  // --- service / utility payments ---
  billers() {
    return request<{ billers: Biller[] }>('/api/billers')
  },
  payService(input: { billerId: string; reference: string; amount: number; currency: Currency }) {
    return request<{ id: string; newBalance: number; simulated: boolean }>('/api/payments/service', {
      method: 'POST',
      body: body(input),
      ...idem(),
    })
  },

  // --- merchants (commerce rail) ---
  listMerchants() {
    return request<{ merchants: Merchant[] }>('/api/merchants')
  },
  createMerchant(input: { name: string; category: string; legalName?: string; idType?: string; idNumber?: string }) {
    return request<Merchant>('/api/merchants', { method: 'POST', body: body(input) })
  },
  merchantCharge(id: string, input: { amount?: number; currency: Currency; description: string }) {
    return request<{ id: string; currency: Currency }>(`/api/merchants/${id}/charge`, { method: 'POST', body: body(input) })
  },

  // --- admin (server-side roles; client only learns its capabilities) ---
  adminWhoami() {
    return request<{ role: string; capabilities: Capabilities }>('/api/admin/whoami')
  },
  adminListMerchants() {
    return request<{ merchants: Merchant[] }>('/api/admin/merchants')
  },
  adminVerifyMerchant(id: string) {
    return request<{ status: string }>(`/api/admin/merchants/${id}/verify`, { method: 'POST', body: '{}' })
  },
  adminRejectMerchant(id: string, reason: string) {
    return request<{ status: string }>(`/api/admin/merchants/${id}/reject`, { method: 'POST', body: body({ reason }) })
  },
  adminSetCommission(id: string, bps: number) {
    return request<{ commissionBps: number }>(`/api/admin/merchants/${id}/commission`, { method: 'POST', body: body({ bps }) })
  },

  // --- admin: users / staff ---
  adminListUsers(params: { q?: string; limit?: number; offset?: number } = {}) {
    const qs = new URLSearchParams()
    if (params.q) qs.set('q', params.q)
    if (params.limit != null) qs.set('limit', String(params.limit))
    if (params.offset != null) qs.set('offset', String(params.offset))
    return request<{ users: AdminUser[]; total: number; limit: number; offset: number; roles: string[] }>(
      `/api/admin/users?${qs.toString()}`,
    )
  },
  adminCreateUser(input: { email: string; fullName: string; phone?: string; role: string; password: string }) {
    return request<AdminUser>('/api/admin/users', { method: 'POST', body: body(input) })
  },
  adminSetRole(id: string, role: string) {
    return request<{ role: string }>(`/api/admin/users/${id}/role`, { method: 'POST', body: body({ role }) })
  },
  adminSetStatus(id: string, disabled: boolean) {
    return request<{ disabled: boolean }>(`/api/admin/users/${id}/status`, { method: 'POST', body: body({ disabled }) })
  },

  // --- admin: reports / analytics ---
  reportOverview() {
    return request<ReportOverview>('/api/admin/reports/overview')
  },
  reportTimeseries(metric: 'count' | 'volume' | 'fees', days: number, currency?: Currency) {
    const qs = new URLSearchParams({ metric, days: String(days) })
    if (currency) qs.set('currency', currency)
    return request<{ series: SeriesPoint[] }>(`/api/admin/reports/timeseries?${qs.toString()}`)
  },
  reportByKind(days: number) {
    return request<{ byKind: ByKindRow[] }>(`/api/admin/reports/by-kind?days=${days}`)
  },
  reportLedgerHealth() {
    return request<LedgerHealth>('/api/admin/reports/ledger-health')
  },
  reportTransactions(params: {
    from?: string
    to?: string
    kind?: string
    currency?: string
    q?: string
    limit?: number
    offset?: number
  }) {
    const qs = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => {
      if (v != null && v !== '') qs.set(k, String(v))
    })
    return request<{ transactions: TxReportRow[]; total: number; limit: number; offset: number }>(
      `/api/admin/reports/transactions?${qs.toString()}`,
    )
  },
  async downloadTransactionsCsv(params: { from?: string; to?: string; kind?: string; currency?: string; q?: string }) {
    const qs = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => {
      if (v != null && v !== '') qs.set(k, String(v))
    })
    const url = `${API_URL}/api/admin/reports/transactions.csv?${qs.toString()}`
    const doFetch = () => fetch(url, { headers: tokens.access ? { Authorization: `Bearer ${tokens.access}` } : {} })
    let res = await doFetch()
    // Mirror request(): refresh the access token once on 401 and retry (a GET, so safe).
    if (res.status === 401 && (await refreshTokens())) {
      res = await doFetch()
    }
    if (!res.ok) throw new ApiError(res.status, `Error ${res.status}`)
    const blob = await res.blob()
    const objectUrl = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = objectUrl
    link.download = 'transacciones.csv'
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(objectUrl)
  },

  // --- admin: audit trail ---
  adminAuditLog(params: { action?: string; limit?: number; offset?: number } = {}) {
    const qs = new URLSearchParams()
    if (params.action) qs.set('action', params.action)
    if (params.limit != null) qs.set('limit', String(params.limit))
    if (params.offset != null) qs.set('offset', String(params.offset))
    return request<{ entries: AuditEntry[]; total: number; limit: number; offset: number }>(
      `/api/admin/audit?${qs.toString()}`,
    )
  },
}

export { ApiError }
