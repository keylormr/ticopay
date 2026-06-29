import { useEffect, useState } from 'react'
import { ApiError, api, type Capabilities, type Merchant } from '../../api'
import { useI18n } from '../../i18n'

export function ComerciosAdmin({ caps }: { caps: Capabilities }) {
  const { t } = useI18n()
  const [merchants, setMerchants] = useState<Merchant[]>([])
  const [error, setError] = useState('')

  function load() {
    api
      .adminListMerchants()
      .then((r) => setMerchants(r.merchants))
      .catch(() => {})
  }
  useEffect(load, [])

  return (
    <section className="panel">
      <h2>{t('admin.title')}</h2>
      <p className="sub">{t('admin.sub')}</p>
      {error && <div className="error">{error}</div>}
      {merchants.length === 0 && <div className="empty">{t('admin.empty')}</div>}
      {merchants.map((m) => (
        <MerchantRow key={m.id} merchant={m} caps={caps} reload={load} onError={setError} />
      ))}
    </section>
  )
}

function MerchantRow({
  merchant,
  caps,
  reload,
  onError,
}: {
  merchant: Merchant
  caps: Capabilities
  reload: () => void
  onError: (s: string) => void
}) {
  const { t } = useI18n()
  const [bps, setBps] = useState(String(merchant.commissionBps))
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)

  async function run(fn: () => Promise<unknown>) {
    setBusy(true)
    onError('')
    try {
      await fn()
      reload()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : t('admin.err'))
    } finally {
      setBusy(false)
    }
  }

  const readOnly = !caps.verifyMerchants && !caps.setCommission

  return (
    <div className="req-row" style={{ flexDirection: 'column', alignItems: 'stretch' }}>
      <div className="tx-meta">
        <div className="name">
          {merchant.name} <span className={`pill pill-${merchant.status}`}>{t(`merch.status.${merchant.status}`)}</span>
        </div>
        <div className="desc">
          {merchant.ownerEmail} · {merchant.category || '—'} · {t('merch.commission', { pct: (merchant.commissionBps / 100).toFixed(2) })}
          {merchant.idNumber ? ` · ${merchant.idType} ${merchant.idNumber}` : ''}
        </div>
      </div>
      {!readOnly && (
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginTop: 8, alignItems: 'center' }}>
          {caps.verifyMerchants && (
            <>
              <button
                className="btn-pay"
                disabled={busy || merchant.status === 'verified'}
                onClick={() => run(() => api.adminVerifyMerchant(merchant.id))}
              >
                {t('admin.verify')}
              </button>
              <input
                className="mini-input"
                style={{ width: 150 }}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder={t('admin.reject.ph')}
              />
              <button className="btn-ghost" disabled={busy} onClick={() => run(() => api.adminRejectMerchant(merchant.id, reason))}>
                {t('admin.reject')}
              </button>
            </>
          )}
          {caps.setCommission && (
            <>
              <input
                className="mini-input"
                style={{ width: 90 }}
                type="number"
                min="0"
                max="10000"
                value={bps}
                onChange={(e) => setBps(e.target.value)}
              />
              <span className="sub">bps</span>
              <button className="btn-ghost" disabled={busy} onClick={() => run(() => api.adminSetCommission(merchant.id, Number(bps)))}>
                {t('admin.setCommission')}
              </button>
            </>
          )}
        </div>
      )}
    </div>
  )
}
