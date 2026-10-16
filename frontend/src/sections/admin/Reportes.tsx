import { useEffect, useMemo, useState } from 'react'
import { api, type ByKindRow, type LedgerHealth, type ReportOverview, type SeriesPoint, type TxReportRow } from '../../api'
import { useI18n } from '../../i18n'
import { formatMoney } from '../../format'
import { AreaChart, BarList, Donut, Kpi, KpiGrid, CHART_COLORS } from '../../components/Charts'

type Metric = 'count' | 'volume' | 'fees'

export function Reportes() {
  const { t } = useI18n()
  const [overview, setOverview] = useState<ReportOverview | null>(null)
  const [byKind, setByKind] = useState<ByKindRow[]>([])
  const [health, setHealth] = useState<LedgerHealth | null>(null)
  const [days, setDays] = useState(30)

  useEffect(() => {
    api.reportOverview().then(setOverview).catch(() => {})
    api.reportLedgerHealth().then(setHealth).catch(() => {})
  }, [])
  useEffect(() => {
    api.reportByKind(days).then((r) => setByKind(r.byKind)).catch(() => {})
  }, [days])

  const crcVolume = overview?.volumeByCurrency.find((v) => v.currency === 'CRC')?.amountCents ?? 0
  const crcFees = overview?.feesByCurrency.find((v) => v.currency === 'CRC')?.amountCents ?? 0

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <section className="panel">
        <h2>{t('rep.title')}</h2>
        <p className="sub">{t('rep.sub')}</p>
        <KpiGrid>
          <Kpi label={t('rep.kpi.users')} value={String(overview?.users.total ?? '—')} sub={t('rep.kpi.active', { n: overview?.users.active30d ?? 0 })} accent={CHART_COLORS[0]} />
          <Kpi label={t('rep.kpi.merchants')} value={String(overview?.merchants?.verified ?? 0)} sub={t('rep.kpi.pending', { n: overview?.merchants?.pending ?? 0 })} accent={CHART_COLORS[2]} />
          <Kpi label={t('rep.kpi.tx')} value={String(overview?.transactions.total ?? '—')} accent={CHART_COLORS[4]} />
          <Kpi label={t('rep.kpi.volume')} value={formatMoney(crcVolume, 'CRC')} sub="CRC" accent={CHART_COLORS[5]} />
          <Kpi label={t('rep.kpi.fees')} value={formatMoney(crcFees, 'CRC')} sub="CRC" accent={CHART_COLORS[1]} />
        </KpiGrid>
      </section>

      <TimeseriesCard />

      <div className="grid">
        <div className="col">
          <section className="panel">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <h2 style={{ margin: 0 }}>{t('rep.byKind')}</h2>
              <DaysToggle days={days} setDays={setDays} />
            </div>
            <div style={{ marginTop: 14 }}>
              <BarList data={byKind.map((k) => ({ label: k.kind, value: k.count }))} formatValue={(n) => n.toLocaleString()} />
            </div>
          </section>
        </div>
        <div className="col">
          <section className="panel">
            <h2>{t('rep.byCurrency')}</h2>
            <p className="sub">{t('rep.byCurrency.sub')}</p>
            <div style={{ marginTop: 8 }}>
              <Donut data={(overview?.volumeByCurrency ?? []).slice(0, 6).map((v) => ({ label: v.currency, value: v.count }))} />
            </div>
          </section>
        </div>
      </div>

      <LedgerHealthCard health={health} />
      <TxTable />
    </div>
  )
}

function DaysToggle({ days, setDays }: { days: number; setDays: (d: number) => void }) {
  return (
    <div style={{ display: 'flex', gap: 6 }}>
      {[7, 30, 90].map((d) => (
        <button key={d} className={d === days ? 'btn-pay' : 'btn-ghost'} style={{ padding: '4px 10px' }} onClick={() => setDays(d)}>
          {d}d
        </button>
      ))}
    </div>
  )
}

function TimeseriesCard() {
  const { t } = useI18n()
  const [metric, setMetric] = useState<Metric>('count')
  const [days, setDays] = useState(30)
  const [series, setSeries] = useState<SeriesPoint[]>([])

  useEffect(() => {
    // For value-based metrics, anchor on CRC so amounts share a unit.
    api
      .reportTimeseries(metric, days, metric === 'count' ? undefined : 'CRC')
      .then((r) => setSeries(r.series))
      .catch(() => {})
  }, [metric, days])

  const color = metric === 'fees' ? CHART_COLORS[1] : metric === 'volume' ? CHART_COLORS[5] : CHART_COLORS[0]
  const fmt = useMemo(
    () => (metric === 'count' ? (n: number) => n.toLocaleString() : (n: number) => formatMoney(n, 'CRC')),
    [metric],
  )

  return (
    <section className="panel">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
        <h2 style={{ margin: 0 }}>{t('rep.activity')}</h2>
        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
          {(['count', 'volume', 'fees'] as Metric[]).map((m) => (
            <button key={m} className={m === metric ? 'btn-pay' : 'btn-ghost'} style={{ padding: '4px 10px' }} onClick={() => setMetric(m)}>
              {t(`rep.metric.${m}`)}
            </button>
          ))}
          <DaysToggle days={days} setDays={setDays} />
        </div>
      </div>
      {metric !== 'count' && <p className="sub" style={{ marginTop: 6 }}>CRC</p>}
      <div style={{ marginTop: 10 }}>
        <AreaChart data={series} color={color} formatValue={fmt} />
      </div>
    </section>
  )
}

function LedgerHealthCard({ health }: { health: LedgerHealth | null }) {
  const { t } = useI18n()
  if (!health) return null
  return (
    <section className="panel">
      <h2>
        {t('rep.ledger')}{' '}
        <span className={`pill ${health.balanced ? 'pill-verified' : 'pill-rejected'}`}>
          {health.balanced ? t('rep.ledger.ok') : t('rep.ledger.bad')}
        </span>
      </h2>
      <p className="sub">{t('rep.ledger.sub')}</p>
      <div className="grid" style={{ marginTop: 8 }}>
        <div className="col">
          <strong style={{ fontSize: 13 }}>{t('rep.ledger.net')}</strong>
          {health.netByCurrency.map((n) => (
            <div key={n.currency} className="desc" style={{ display: 'flex', justifyContent: 'space-between' }}>
              <span>{n.currency}</span>
              <span style={{ color: n.netCents === 0 ? 'inherit' : '#E03131' }}>{n.netCents}</span>
            </div>
          ))}
        </div>
        <div className="col">
          <strong style={{ fontSize: 13 }}>{t('rep.ledger.system')}</strong>
          {health.systemAccounts.map((s, i) => (
            <div key={i} className="desc" style={{ display: 'flex', justifyContent: 'space-between' }}>
              <span>
                {s.account} · {s.currency}
              </span>
              <span>{formatMoney(s.balanceCents, s.currency)}</span>
            </div>
          ))}
        </div>
      </div>
      {health.driftTotalCents > 0 && (
        <div className="desc" style={{ marginTop: 10, color: '#E03131', fontWeight: 600 }}>
          {t('rep.ledger.drift')}: {health.driftTotalCents} ({health.drift.length})
        </div>
      )}
    </section>
  )
}

const KINDS = ['transfer', 'conversion', 'request', 'merchant', 'pool', 'service', 'sinpe']

function TxTable() {
  const { t } = useI18n()
  const [rows, setRows] = useState<TxReportRow[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [kind, setKind] = useState('')
  const [q, setQ] = useState('')
  const [csvBusy, setCsvBusy] = useState(false)
  const [csvErr, setCsvErr] = useState('')
  const limit = 25

  async function downloadCsv() {
    setCsvErr('')
    setCsvBusy(true)
    try {
      await api.downloadTransactionsCsv(filters)
    } catch {
      setCsvErr(t('rep.tx.csvErr'))
    } finally {
      setCsvBusy(false)
    }
  }

  const filters = useMemo(() => ({ from, to, kind, q }), [from, to, kind, q])

  function load() {
    api
      .reportTransactions({ ...filters, limit, offset })
      .then((r) => {
        setRows(r.transactions)
        setTotal(r.total)
      })
      .catch(() => {})
  }
  useEffect(load, [from, to, kind, q, offset])

  return (
    <section className="panel">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
        <h2 style={{ margin: 0 }}>{t('rep.tx')}</h2>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {csvErr && <span className="error" style={{ margin: 0 }}>{csvErr}</span>}
          <button className="btn-ghost" disabled={csvBusy} onClick={downloadCsv}>
            {csvBusy ? '…' : t('rep.tx.csv')}
          </button>
        </div>
      </div>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', margin: '10px 0' }}>
        <input type="date" value={from} onChange={(e) => { setOffset(0); setFrom(e.target.value) }} />
        <input type="date" value={to} onChange={(e) => { setOffset(0); setTo(e.target.value) }} />
        <select value={kind} onChange={(e) => { setOffset(0); setKind(e.target.value) }}>
          <option value="">{t('rep.tx.allKinds')}</option>
          {KINDS.map((k) => (
            <option key={k} value={k}>
              {k}
            </option>
          ))}
        </select>
        <input value={q} onChange={(e) => { setOffset(0); setQ(e.target.value) }} placeholder={t('rep.tx.search')} />
      </div>

      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
          <thead>
            <tr style={{ textAlign: 'left', opacity: 0.7 }}>
              <th style={{ padding: '6px 8px' }}>{t('rep.tx.date')}</th>
              <th style={{ padding: '6px 8px' }}>{t('rep.tx.kind')}</th>
              <th style={{ padding: '6px 8px' }}>{t('rep.tx.from')}</th>
              <th style={{ padding: '6px 8px' }}>{t('rep.tx.to')}</th>
              <th style={{ padding: '6px 8px', textAlign: 'right' }}>{t('rep.tx.amount')}</th>
              <th style={{ padding: '6px 8px', textAlign: 'right' }}>{t('rep.tx.fee')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.id} style={{ borderTop: '1px solid rgba(0,0,0,0.06)' }}>
                <td style={{ padding: '6px 8px', whiteSpace: 'nowrap' }}>{new Date(r.createdAt).toLocaleString()}</td>
                <td style={{ padding: '6px 8px', textTransform: 'capitalize' }}>{r.kind}</td>
                <td style={{ padding: '6px 8px' }}>{r.fromEmail || '—'}</td>
                <td style={{ padding: '6px 8px' }}>{r.toEmail || '—'}</td>
                <td style={{ padding: '6px 8px', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>{formatMoney(r.amountCents, r.currency)}</td>
                <td style={{ padding: '6px 8px', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
                  {r.feeCents > 0 ? formatMoney(r.feeCents, r.currency) : '—'}
                </td>
              </tr>
            ))}
            {rows.length === 0 && (
              <tr>
                <td colSpan={6} style={{ padding: '16px 8px', textAlign: 'center', opacity: 0.5 }}>
                  {t('rep.tx.empty')}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {total > limit && (
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 12 }}>
          <button className="btn-ghost" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}>
            {t('users.prev')}
          </button>
          <span className="sub">
            {offset + 1}–{Math.min(offset + limit, total)} / {total}
          </span>
          <button className="btn-ghost" disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}>
            {t('users.next')}
          </button>
        </div>
      )}
    </section>
  )
}
