import { lazy, Suspense, useState } from 'react'
import { api, type Account } from '../api'
import { useAuth } from '../auth'
import { useRates } from '../rates'
import { useI18n } from '../i18n'
import { Brand } from '../components/Brand'
import { CoinLogo } from '../components/CoinLogo'
import { LangToggle } from '../components/LangToggle'
import { CRYPTO, FIAT, metaOf } from '../currencies'
import { formatMoney } from '../format'
import { Movimientos } from '../sections/Movimientos'

// Only the "inicio" tab (and its Movimientos list) loads up front. Every other
// tab — including the admin panel with its charts and the QR/WebAuthn code — is
// code-split and fetched the first time it's opened, so the average user who
// never leaves the home tab downloads far less on the first load.
const Sinpe = lazy(() => import('../sections/Sinpe').then((m) => ({ default: m.Sinpe })))
const Convertir = lazy(() => import('../sections/Convertir').then((m) => ({ default: m.Convertir })))
const SendMoney = lazy(() => import('../sections/SendMoney').then((m) => ({ default: m.SendMoney })))
const Cobros = lazy(() => import('../sections/Cobros').then((m) => ({ default: m.Cobros })))
const Servicios = lazy(() => import('../sections/Servicios').then((m) => ({ default: m.Servicios })))
const Vaquitas = lazy(() => import('../sections/Vaquitas').then((m) => ({ default: m.Vaquitas })))
const Comercio = lazy(() => import('../sections/Comercio').then((m) => ({ default: m.Comercio })))
const Admin = lazy(() => import('../sections/Admin').then((m) => ({ default: m.Admin })))
const AccountTab = lazy(() => import('../sections/Account').then((m) => ({ default: m.Account })))

type Tab = 'inicio' | 'sinpe' | 'convertir' | 'enviar' | 'cobrar' | 'servicios' | 'vaquitas' | 'comercio' | 'cuenta' | 'admin'

const TABS: { id: Tab; icon: string }[] = [
  { id: 'inicio', icon: '🏠' },
  { id: 'sinpe', icon: '📲' },
  { id: 'convertir', icon: '🔄' },
  { id: 'enviar', icon: '💸' },
  { id: 'cobrar', icon: '🧾' },
  { id: 'servicios', icon: '💡' },
  { id: 'vaquitas', icon: '🐮' },
  { id: 'comercio', icon: '🏪' },
  { id: 'cuenta', icon: '👤' },
]

const ADMIN_TAB: { id: Tab; icon: string } = { id: 'admin', icon: '🛡️' }

export function Dashboard() {
  const { user, accounts, caps, logout, refresh } = useAuth()
  const { rates } = useRates()
  const { t } = useI18n()
  const [tab, setTab] = useState<Tab>('inicio')
  const [version, setVersion] = useState(0)
  const [showAllCrypto, setShowAllCrypto] = useState(false)

  // rates come from the shared RatesProvider (loaded in parallel with the auth
  // me() call) and capabilities ride along in that me() response, so the
  // dashboard no longer fires its own rates fetch or a separate /admin/whoami
  // round-trip on every mount. The admin tab still gates on server-derived
  // capabilities; every admin endpoint is enforced server-side regardless.
  const visibleTabs = caps?.backoffice ? [...TABS, ADMIN_TAB] : TABS

  async function reload() {
    await refresh()
    setVersion((v) => v + 1)
  }

  const accountOf = (code: string) => accounts.find((a) => a.currency === code)

  function valueCrc(a: Account): number | null {
    if (!rates) return null
    const major = a.balanceCents / 10 ** metaOf(a.currency).decimals
    const usd = major * (rates.usdPerUnit[a.currency] ?? 0)
    return rates.crc?.sell ? usd * rates.crc.sell : null
  }

  const totalUsd = rates
    ? accounts.reduce(
        (sum, a) => sum + (a.balanceCents / 10 ** metaOf(a.currency).decimals) * (rates.usdPerUnit[a.currency] ?? 0),
        0,
      )
    : null
  const totalCrc = totalUsd != null && rates?.crc?.sell ? totalUsd * rates.crc.sell : null

  const cryptoWallets = CRYPTO.map((c) => accountOf(c.code)).filter((a): a is Account => Boolean(a))
  const nonZeroCrypto = cryptoWallets.filter((a) => a.balanceCents > 0)
  const shownCrypto = showAllCrypto ? cryptoWallets : nonZeroCrypto

  function changeTab(id: Tab) {
    setTab(id)
    window.scrollTo({ top: 0 })
  }

  return (
    <>
      <header className="topbar">
        <Brand />
        <div className="who">
          {totalCrc != null && <span className="total-chip">{formatMoney(Math.round(totalCrc * 100), 'CRC')}</span>}
          <LangToggle />
          <span className="who-name">{user?.fullName}</span>
          {user?.kycStatus === 'verified' && <span className="badge-verified">✓</span>}
          <button className="btn-ghost" onClick={logout}>
            {t('btn.signout')}
          </button>
        </div>
      </header>

      <nav className="tabs sticky-tabs">
        {visibleTabs.map((tb) => (
          <button key={tb.id} className={`tab ${tab === tb.id ? 'tab-active' : ''}`} onClick={() => changeTab(tb.id)}>
            <span className="tab-icon">{tb.icon}</span>
            {t(`tab.${tb.id}`)}
          </button>
        ))}
      </nav>

      <main className="container">
        {user && !user.emailVerified && <VerifyBanner />}
        {tab === 'inicio' && (
          <>
            <section className="hero">
              <div className="hero-label">{t('dash.netWorth')}</div>
              <div className="hero-amount">{totalCrc != null ? formatMoney(Math.round(totalCrc * 100), 'CRC') : '—'}</div>
              {totalUsd != null && <div className="hero-sub">≈ {formatMoney(Math.round(totalUsd * 100), 'USD')}</div>}
            </section>

            <h3 className="section-title">{t('dash.myMoney')}</h3>
            <div className="fiat-cards">
              {FIAT.map((c) => {
                const a = accountOf(c.code)
                if (!a) return null
                return (
                  <div className="fiat-card" key={c.code} style={{ borderTopColor: c.color }}>
                    <CoinLogo code={c.code} />
                    <div className="fc-body">
                      <div className="fc-name">
                        {c.name} <span className="fc-code">{c.code}</span>
                      </div>
                      <div className="fc-amount">{formatMoney(a.balanceCents, c.code)}</div>
                    </div>
                  </div>
                )
              })}
            </div>

            <div className="section-head">
              <h3 className="section-title">{t('dash.myCrypto')}</h3>
              <button className="link-btn" onClick={() => setShowAllCrypto((s) => !s)}>
                {showAllCrypto ? t('dash.seeWithBalance') : t('dash.seeAll', { n: cryptoWallets.length })}
              </button>
            </div>
            <div className="panel coin-panel">
              {shownCrypto.length === 0 ? (
                <div className="empty">
                  {t('dash.noCrypto')}{' '}
                  <button className="link-btn" onClick={() => setShowAllCrypto(true)}>
                    {t('dash.seeAvailable')}
                  </button>
                </div>
              ) : (
                <ul className="coin-list">
                  {shownCrypto.map((a) => {
                    const m = metaOf(a.currency)
                    const v = valueCrc(a)
                    return (
                      <li className="coin-row" key={a.id}>
                        <CoinLogo code={a.currency} />
                        <div className="coin-info">
                          <div className="coin-name">{m.name}</div>
                          <div className="coin-ticker">{a.currency}</div>
                        </div>
                        <div className="coin-bal">
                          <div className="coin-amount">{formatMoney(a.balanceCents, a.currency)}</div>
                          {v != null && a.balanceCents > 0 && (
                            <div className="coin-fiat">≈ {formatMoney(Math.round(v * 100), 'CRC')}</div>
                          )}
                        </div>
                      </li>
                    )
                  })}
                </ul>
              )}
            </div>

            <Movimientos version={version} />
          </>
        )}

        {tab !== 'inicio' && (
          <Suspense fallback={<div className="section-loading">{t('common.loading')}</div>}>
            {tab === 'sinpe' && <Sinpe reload={reload} />}
            {tab === 'convertir' && <Convertir reload={reload} />}
            {tab === 'enviar' && <SendMoney reload={reload} />}
            {tab === 'cobrar' && <Cobros version={version} reload={reload} />}
            {tab === 'servicios' && <Servicios reload={reload} />}
            {tab === 'vaquitas' && <Vaquitas version={version} reload={reload} />}
            {tab === 'comercio' && <Comercio reload={reload} />}
            {tab === 'cuenta' && <AccountTab />}
            {tab === 'admin' && caps?.backoffice && <Admin caps={caps} />}
          </Suspense>
        )}
      </main>
    </>
  )
}

function VerifyBanner() {
  const { t } = useI18n()
  const [sent, setSent] = useState(false)
  const [busy, setBusy] = useState(false)

  async function resend() {
    setBusy(true)
    try {
      await api.resendVerification()
      setSent(true)
    } catch {
      /* ignore — banner is best-effort */
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="verify-banner">
      <span>📧 {t('verify.banner')}</span>
      {sent ? (
        <span className="verify-sent">{t('verify.banner.sent')}</span>
      ) : (
        <button className="link-btn" onClick={resend} disabled={busy}>
          {busy ? t('auth.processing') : t('verify.banner.resend')}
        </button>
      )}
    </div>
  )
}
