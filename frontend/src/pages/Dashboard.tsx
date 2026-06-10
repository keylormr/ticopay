import { useEffect, useState } from 'react'
import { api, type Account, type Rates } from '../api'
import { useAuth } from '../auth'
import { useI18n } from '../i18n'
import { Brand } from '../components/Brand'
import { CoinLogo } from '../components/CoinLogo'
import { LangToggle } from '../components/LangToggle'
import { CRYPTO, FIAT, metaOf } from '../currencies'
import { formatMoney } from '../format'
import { Movimientos } from '../sections/Movimientos'
import { Sinpe } from '../sections/Sinpe'
import { Convertir } from '../sections/Convertir'
import { SendMoney } from '../sections/SendMoney'
import { Cobros } from '../sections/Cobros'
import { Servicios } from '../sections/Servicios'
import { Vaquitas } from '../sections/Vaquitas'
import { Account as AccountTab } from '../sections/Account'

type Tab = 'inicio' | 'sinpe' | 'convertir' | 'enviar' | 'cobrar' | 'servicios' | 'vaquitas' | 'cuenta'

const TABS: { id: Tab; icon: string }[] = [
  { id: 'inicio', icon: '🏠' },
  { id: 'sinpe', icon: '📲' },
  { id: 'convertir', icon: '🔄' },
  { id: 'enviar', icon: '💸' },
  { id: 'cobrar', icon: '🧾' },
  { id: 'servicios', icon: '💡' },
  { id: 'vaquitas', icon: '🐮' },
  { id: 'cuenta', icon: '👤' },
]

export function Dashboard() {
  const { user, accounts, logout, refresh } = useAuth()
  const { t } = useI18n()
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
        {TABS.map((tb) => (
          <button key={tb.id} className={`tab ${tab === tb.id ? 'tab-active' : ''}`} onClick={() => changeTab(tb.id)}>
            <span className="tab-icon">{tb.icon}</span>
            {t(`tab.${tb.id}`)}
          </button>
        ))}
      </nav>

      <main className="container">
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

        {tab === 'sinpe' && <Sinpe reload={reload} />}
        {tab === 'convertir' && <Convertir reload={reload} />}
        {tab === 'enviar' && <SendMoney reload={reload} />}
        {tab === 'cobrar' && <Cobros version={version} reload={reload} />}
        {tab === 'servicios' && <Servicios reload={reload} />}
        {tab === 'vaquitas' && <Vaquitas version={version} reload={reload} />}
        {tab === 'cuenta' && <AccountTab />}
      </main>
    </>
  )
}
