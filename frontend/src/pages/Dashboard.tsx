import { useEffect, useState } from 'react'
import { api, type Rates } from '../api'
import { useAuth } from '../auth'
import { Brand } from '../components/Brand'
import { CURRENCIES, metaOf } from '../currencies'
import { formatMoney } from '../format'
import { Home } from '../sections/Home'
import { SendMoney } from '../sections/SendMoney'
import { Cobros } from '../sections/Cobros'
import { Vaquitas } from '../sections/Vaquitas'
import { Account } from '../sections/Account'

type Tab = 'inicio' | 'enviar' | 'cobrar' | 'vaquitas' | 'cuenta'

const TABS: { id: Tab; label: string }[] = [
  { id: 'inicio', label: 'Inicio' },
  { id: 'enviar', label: 'Enviar' },
  { id: 'cobrar', label: 'Cobrar' },
  { id: 'vaquitas', label: 'Vaquitas' },
  { id: 'cuenta', label: 'Cuenta' },
]

export function Dashboard() {
  const { user, accounts, logout, refresh } = useAuth()
  const [tab, setTab] = useState<Tab>('inicio')
  const [version, setVersion] = useState(0)
  const [rates, setRates] = useState<Rates | null>(null)

  useEffect(() => {
    api.rates().then(setRates).catch(() => {})
  }, [version])

  async function reload() {
    await refresh()
    setVersion((v) => v + 1)
  }

  // Wallets ordered by the currency catalog (fiat first, then crypto).
  const wallets = CURRENCIES.map((c) => accounts.find((a) => a.currency === c.code)).filter(
    (a): a is NonNullable<typeof a> => Boolean(a),
  )

  const totalUsd = rates
    ? accounts.reduce(
        (sum, a) => sum + (a.balanceCents / 10 ** metaOf(a.currency).decimals) * (rates.usdPerUnit[a.currency] ?? 0),
        0,
      )
    : null
  const totalCrc = totalUsd != null && rates?.crc?.sell ? totalUsd * rates.crc.sell : null

  return (
    <>
      <header className="topbar">
        <Brand />
        <div className="who">
          <span>{user?.fullName}</span>
          {user?.kycStatus === 'verified' && <span className="badge-verified">✓ Verificado</span>}
          <button className="btn-ghost" onClick={logout}>
            Salir
          </button>
        </div>
      </header>

      <main className="container">
        {totalCrc != null && (
          <div className="net-worth">
            Patrimonio estimado: <strong>{formatMoney(Math.round(totalCrc * 100), 'CRC')}</strong>
            <span className="nw-usd"> · {formatMoney(Math.round((totalUsd ?? 0) * 100), 'USD')}</span>
          </div>
        )}

        <section className="wallets">
          {wallets.map((a) => {
            const m = metaOf(a.currency)
            return (
              <div key={a.id} className={`wallet ${m.type}`}>
                <div className="w-top">
                  <span className="w-name">
                    {m.symbol} {a.currency}
                  </span>
                  <span className={`w-tag w-${m.type}`}>{m.type === 'crypto' ? 'Cripto' : 'Fiat'}</span>
                </div>
                <div className="w-amount">{formatMoney(a.balanceCents, a.currency)}</div>
                {m.type === 'crypto' && rates?.crypto?.[a.currency] != null && (
                  <div className="w-sub">1 {a.currency} ≈ ${rates.crypto[a.currency].toLocaleString('en-US')}</div>
                )}
              </div>
            )
          })}
        </section>

        <nav className="tabs">
          {TABS.map((t) => (
            <button key={t.id} className={`tab ${tab === t.id ? 'tab-active' : ''}`} onClick={() => setTab(t.id)}>
              {t.label}
            </button>
          ))}
        </nav>

        <div className="tab-body">
          {tab === 'inicio' && <Home version={version} reload={reload} />}
          {tab === 'enviar' && <SendMoney reload={reload} />}
          {tab === 'cobrar' && <Cobros version={version} reload={reload} />}
          {tab === 'vaquitas' && <Vaquitas version={version} reload={reload} />}
          {tab === 'cuenta' && <Account />}
        </div>
      </main>
    </>
  )
}
