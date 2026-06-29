import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, type Currency, type Merchant } from '../api'
import { useI18n } from '../i18n'
import { ShareCard } from '../components/ShareCard'
import { CurrencySelect } from '../components/CurrencySelect'

export function Comercio({ reload }: { reload: () => Promise<void> }) {
  const { t } = useI18n()
  const [merchants, setMerchants] = useState<Merchant[]>([])
  const [name, setName] = useState('')
  const [category, setCategory] = useState('')
  const [legalName, setLegalName] = useState('')
  const [idType, setIdType] = useState('')
  const [idNumber, setIdNumber] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  function load() {
    api
      .listMerchants()
      .then((r) => setMerchants(r.merchants))
      .catch(() => {})
  }
  useEffect(load, [])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (!name.trim()) {
      setError(t('merch.err.name'))
      return
    }
    setBusy(true)
    try {
      await api.createMerchant({
        name: name.trim(),
        category: category.trim(),
        legalName: legalName.trim() || undefined,
        idType: idType || undefined,
        idNumber: idNumber.trim() || undefined,
      })
      setName('')
      setCategory('')
      setLegalName('')
      setIdType('')
      setIdNumber('')
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('merch.err.create'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid">
      <div className="col">
        <section className="panel">
          <h2>{t('merch.register')}</h2>
          <p className="sub">{t('merch.register.sub')}</p>
          <form onSubmit={onCreate}>
            <label htmlFor="mname">{t('merch.name')}</label>
            <input id="mname" value={name} onChange={(e) => setName(e.target.value)} placeholder={t('merch.name.ph')} />
            <label htmlFor="mcat">{t('merch.category')}</label>
            <input id="mcat" value={category} onChange={(e) => setCategory(e.target.value)} placeholder={t('merch.category.ph')} />
            <label htmlFor="mlegal">{t('merch.legalName')}</label>
            <input id="mlegal" value={legalName} onChange={(e) => setLegalName(e.target.value)} placeholder={t('merch.legalName.ph')} />
            <label htmlFor="midtype">{t('merch.idType')}</label>
            <select id="midtype" value={idType} onChange={(e) => setIdType(e.target.value)}>
              <option value="">{t('merch.idType.none')}</option>
              <option value="fisica">{t('acct.idType.fisica')}</option>
              <option value="juridica">{t('acct.idType.juridica')}</option>
              <option value="dimex">{t('acct.idType.dimex')}</option>
            </select>
            <label htmlFor="midnum">{t('merch.idNumber')}</label>
            <input id="midnum" value={idNumber} onChange={(e) => setIdNumber(e.target.value)} placeholder={t('merch.idNumber.ph')} />
            {error && <div className="error">{error}</div>}
            <button className="btn" type="submit" disabled={busy}>
              {busy ? t('merch.busy') : t('merch.btn')}
            </button>
          </form>
        </section>
      </div>

      <div className="col">
        <section className="panel">
          <h2>{t('merch.mine')}</h2>
          <p className="sub">{t('merch.mine.sub')}</p>
          {merchants.length === 0 && <div className="empty">{t('merch.empty')}</div>}
          {merchants.map((m) => (
            <MerchantRow
              key={m.id}
              merchant={m}
              reload={async () => {
                await reload()
                load()
              }}
            />
          ))}
        </section>
      </div>
    </div>
  )
}

function MerchantRow({ merchant }: { merchant: Merchant; reload: () => Promise<void> }) {
  const { t } = useI18n()
  const [amount, setAmount] = useState('')
  const [currency, setCurrency] = useState<Currency>('CRC')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [chargeId, setChargeId] = useState('')

  const pct = (merchant.commissionBps / 100).toFixed(2)

  async function createCharge(e: FormEvent) {
    e.preventDefault()
    setError('')
    setChargeId('')
    setBusy(true)
    try {
      const res = await api.merchantCharge(merchant.id, {
        amount: amount ? Number(amount) : undefined,
        currency,
        description: description.trim(),
      })
      setChargeId(res.id)
      setAmount('')
      setDescription('')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('merch.err.charge'))
    } finally {
      setBusy(false)
    }
  }

  const shareUrl = chargeId ? `${window.location.origin}/cobro/${chargeId}` : ''

  return (
    <div className="req-row" style={{ flexDirection: 'column', alignItems: 'stretch' }}>
      <div className="tx-meta">
        <div className="name">
          {merchant.name} <span className={`pill pill-${merchant.status}`}>{t(`merch.status.${merchant.status}`)}</span>
        </div>
        <div className="desc">
          {merchant.category || t('cobros.noConcept')} · {t('merch.commission', { pct })}
        </div>
        {merchant.status === 'rejected' && merchant.rejectReason && (
          <div className="error" style={{ marginTop: 6 }}>
            {merchant.rejectReason}
          </div>
        )}
      </div>

      {merchant.status === 'verified' && (
        <form onSubmit={createCharge} style={{ marginTop: 10 }}>
          <label>{t('merch.charge.amount')}</label>
          <input
            type="number"
            min="0"
            step="any"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder={t('merch.charge.amount.ph')}
          />
          <CurrencySelect value={currency} onChange={setCurrency} />
          <input value={description} onChange={(e) => setDescription(e.target.value)} placeholder={t('merch.charge.desc.ph')} />
          {error && <div className="error">{error}</div>}
          <button className="btn" type="submit" disabled={busy}>
            {busy ? t('merch.charge.busy') : t('merch.charge.btn')}
          </button>
          {shareUrl && (
            <>
              <div className="ok" style={{ marginTop: 12 }}>
                {t('merch.charge.created')}
              </div>
              <ShareCard url={shareUrl} message={t('merch.shareMsg')} />
            </>
          )}
        </form>
      )}
      {merchant.status === 'pending' && (
        <div className="sub" style={{ marginTop: 6 }}>
          {t('merch.pendingNote')}
        </div>
      )}
    </div>
  )
}
