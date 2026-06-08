import { useState } from 'react'
import { useAuth } from '../auth'
import { Brand } from '../components/Brand'
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

  // Called by any action that moves money — refreshes balances and signals
  // sections to refetch their lists.
  async function reload() {
    await refresh()
    setVersion((v) => v + 1)
  }

  const crc = accounts.find((a) => a.currency === 'CRC')
  const usd = accounts.find((a) => a.currency === 'USD')

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
        <section className="balance-row">
          <div className="balance-card">
            <div className="label">Saldo en colones</div>
            <div className="amount">{crc ? formatMoney(crc.balanceCents, 'CRC') : '—'}</div>
          </div>
          <div className="balance-card usd">
            <div className="label">Saldo en dólares</div>
            <div className="amount">{usd ? formatMoney(usd.balanceCents, 'USD') : '—'}</div>
          </div>
        </section>

        <nav className="tabs">
          {TABS.map((t) => (
            <button
              key={t.id}
              className={`tab ${tab === t.id ? 'tab-active' : ''}`}
              onClick={() => setTab(t.id)}
            >
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
