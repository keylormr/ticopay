import { useEffect, useState } from 'react'
import { api, type Account, type Rates } from '../api'
import { useAuth } from '../auth'
import { Brand } from '../components/Brand'
import { CoinLogo } from '../components/CoinLogo'
import { CRYPTO, FIAT, metaOf } from '../currencies'
import { formatMoney } from '../format'
import { Home } from '../sections/Home'
import { SendMoney } from '../sections/SendMoney'
import { Cobros } from '../sections/Cobros'
import { Vaquitas } from '../sections/Vaquitas'
import { Account as AccountTab } from '../sections/Account'

type Tab = 'inicio' | 'enviar' | 'cobrar' | 'vaquitas' | 'cuenta'

const TABS: { id: Tab; label: string; icon: string }[] = [
  { id: 'inicio', label: 'Inicio', icon: '🏠' },
  { id: 'enviar', label: 'Enviar', icon: '💸' },
  { id: 'cobrar', label: 'Cobrar', icon: '🧾' },
  { id: 'vaquitas', label: 'Vaquitas', icon: '🐮' },
  { id: 'cuenta', label: 'Cuenta', icon: '👤' },
]

export function Dashboard() {
  const { user, accounts, logout, refresh } = useAuth()
  const [tab, setTab] = useState<Tab>('inicio')
  const [version, setVersion] = useState(0)
  const [rates, setRates] = useState<Rates | null>(null)
  const [showAllCrypto, setShowAllCrypto] = useState(false)

  useEffect(() => {
    api.rates().then(setRates).catch(() => {})
  }, [version])

  async function reload() {
    await refresh()
    setVersion((v) => v + 1)
  }

  const accountOf = (code: string) => accounts.find((a) => a.currency === code)

  // Value of a wallet expressed in colones (for the friendly "≈ ₡" line).
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

  return (
    <>
      <header className="topbar">
        <Brand />
        <div className="who">
          <span className="who-name">{user?.fullName}</span>
          {user?.kycStatus === 'verified' && <span className="badge-verified">✓ Verificado</span>}
          <button className="btn-ghost" onClick={logout}>
            Salir
          </button>
        </div>
      </header>

      <main className="container">
        {/* Hero: estimated net worth */}
        <section className="hero">
          <div className="hero-label">Tu dinero en Tico Pay</div>
          <div className="hero-amount">{totalCrc != null ? formatMoney(Math.round(totalCrc * 100), 'CRC') : '—'}</div>
          {totalUsd != null && <div className="hero-sub">≈ {formatMoney(Math.round(totalUsd * 100), 'USD')}</div>}
        </section>

        {/* Fiat */}
        <h3 className="section-title">Mis monedas</h3>
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

        {/* Crypto */}
        <div className="section-head">
          <h3 className="section-title">Mis criptomonedas</h3>
          <button className="link-btn" onClick={() => setShowAllCrypto((s) => !s)}>
            {showAllCrypto ? 'Ver solo con saldo' : `Ver todas (${cryptoWallets.length})`}
          </button>
        </div>
        <div className="panel coin-panel">
          {shownCrypto.length === 0 ? (
            <div className="empty">
              Todavía no tenés cripto. <button className="link-btn" onClick={() => setShowAllCrypto(true)}>Ver monedas disponibles</button>
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

        <nav className="tabs">
          {TABS.map((t) => (
            <button key={t.id} className={`tab ${tab === t.id ? 'tab-active' : ''}`} onClick={() => setTab(t.id)}>
              <span className="tab-icon">{t.icon}</span>
              {t.label}
            </button>
          ))}
        </nav>

        <div className="tab-body">
          {tab === 'inicio' && <Home version={version} reload={reload} />}
          {tab === 'enviar' && <SendMoney reload={reload} />}
          {tab === 'cobrar' && <Cobros version={version} reload={reload} />}
          {tab === 'vaquitas' && <Vaquitas version={version} reload={reload} />}
          {tab === 'cuenta' && <AccountTab />}
        </div>
      </main>
    </>
  )
}
