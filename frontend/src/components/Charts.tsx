import { useId, type ReactNode } from 'react'

// Modern, dependency-free SVG charts themed for Tico Pay. Colors lean on the CR
// palette (blues/reds) with a few accents.
export const CHART_COLORS = ['#2E75B6', '#E03131', '#2F9E44', '#F08C00', '#7048E8', '#0CA678', '#E8590C', '#1098AD']

export function Kpi({ label, value, sub, accent = '#2E75B6' }: { label: string; value: string; sub?: string; accent?: string }) {
  return (
    <div
      style={{
        background: 'var(--card, #fff)',
        border: '1px solid rgba(0,0,0,0.08)',
        borderRadius: 14,
        padding: '14px 16px',
        boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
        borderTop: `3px solid ${accent}`,
        minWidth: 0,
      }}
    >
      <div style={{ fontSize: 12, opacity: 0.65, marginBottom: 6 }}>{label}</div>
      <div style={{ fontSize: 24, fontWeight: 700, lineHeight: 1.1, wordBreak: 'break-word' }}>{value}</div>
      {sub && <div style={{ fontSize: 12, opacity: 0.6, marginTop: 4 }}>{sub}</div>}
    </div>
  )
}

export function KpiGrid({ children }: { children: ReactNode }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: 12 }}>{children}</div>
  )
}

export function AreaChart({
  data,
  height = 180,
  color = '#2E75B6',
  formatValue = (n: number) => String(n),
}: {
  data: { date: string; value: number }[]
  height?: number
  color?: string
  formatValue?: (n: number) => string
}) {
  const gid = useId()
  const W = 680
  const H = height
  const pad = 10
  const topGap = 18
  const n = data.length

  if (n === 0) return <Empty />

  const max = Math.max(1, ...data.map((d) => d.value))
  const x = (i: number) => (n <= 1 ? W / 2 : pad + (i * (W - pad * 2)) / (n - 1))
  const y = (v: number) => H - pad - (v / max) * (H - pad * 2 - topGap)

  const line = data.map((d, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(d.value).toFixed(1)}`).join(' ')
  const area = `${line} L${x(n - 1).toFixed(1)},${(H - pad).toFixed(1)} L${x(0).toFixed(1)},${(H - pad).toFixed(1)} Z`
  const last = data[n - 1]

  const grid = [0.25, 0.5, 0.75, 1].map((f) => {
    const gy = H - pad - f * (H - pad * 2 - topGap)
    return <line key={f} x1={pad} y1={gy} x2={W - pad} y2={gy} stroke="rgba(0,0,0,0.06)" strokeWidth={1} />
  })

  return (
    <svg viewBox={`0 0 ${W} ${H}`} width="100%" height={H} preserveAspectRatio="none" role="img">
      <defs>
        <linearGradient id={gid} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity={0.28} />
          <stop offset="100%" stopColor={color} stopOpacity={0.02} />
        </linearGradient>
      </defs>
      {grid}
      <path d={area} fill={`url(#${gid})`} />
      <path d={line} fill="none" stroke={color} strokeWidth={2.5} strokeLinejoin="round" strokeLinecap="round" />
      <circle cx={x(n - 1)} cy={y(last.value)} r={3.8} fill={color} />
      <text x={W - pad} y={14} textAnchor="end" fontSize={12} fontWeight={700} fill={color}>
        {formatValue(last.value)}
      </text>
    </svg>
  )
}

export function BarList({
  data,
  formatValue = (n: number) => String(n),
}: {
  data: { label: string; value: number; color?: string }[]
  formatValue?: (n: number) => string
}) {
  if (data.length === 0) return <Empty />
  const max = Math.max(1, ...data.map((d) => d.value))
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      {data.map((d, i) => (
        <div key={d.label} style={{ display: 'grid', gridTemplateColumns: '90px 1fr auto', alignItems: 'center', gap: 10 }}>
          <span style={{ fontSize: 13, opacity: 0.8, textTransform: 'capitalize' }}>{d.label}</span>
          <div style={{ background: 'rgba(0,0,0,0.06)', borderRadius: 8, height: 12, overflow: 'hidden' }}>
            <div
              style={{
                width: `${Math.max(2, (d.value / max) * 100)}%`,
                height: '100%',
                borderRadius: 8,
                background: d.color ?? CHART_COLORS[i % CHART_COLORS.length],
              }}
            />
          </div>
          <span style={{ fontSize: 13, fontWeight: 600, fontVariantNumeric: 'tabular-nums' }}>{formatValue(d.value)}</span>
        </div>
      ))}
    </div>
  )
}

export function Donut({
  data,
  size = 160,
  thickness = 20,
}: {
  data: { label: string; value: number; color?: string }[]
  size?: number
  thickness?: number
}) {
  const total = data.reduce((s, d) => s + d.value, 0)
  if (total <= 0) return <Empty />
  const r = (size - thickness) / 2
  const cx = size / 2
  const cy = size / 2
  const circ = 2 * Math.PI * r
  let offset = 0

  return (
    <div style={{ display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap' }}>
      <svg viewBox={`0 0 ${size} ${size}`} width={size} height={size}>
        <circle cx={cx} cy={cy} r={r} fill="none" stroke="rgba(0,0,0,0.06)" strokeWidth={thickness} />
        {data.map((d, i) => {
          const frac = d.value / total
          const dash = frac * circ
          const seg = (
            <circle
              key={d.label}
              cx={cx}
              cy={cy}
              r={r}
              fill="none"
              stroke={d.color ?? CHART_COLORS[i % CHART_COLORS.length]}
              strokeWidth={thickness}
              strokeDasharray={`${dash} ${circ - dash}`}
              strokeDashoffset={-offset}
              transform={`rotate(-90 ${cx} ${cy})`}
            />
          )
          offset += dash
          return seg
        })}
      </svg>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        {data.map((d, i) => (
          <div key={d.label} style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13 }}>
            <span style={{ width: 10, height: 10, borderRadius: 3, background: d.color ?? CHART_COLORS[i % CHART_COLORS.length] }} />
            <span style={{ textTransform: 'capitalize' }}>{d.label}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function Empty() {
  return <div style={{ opacity: 0.5, fontSize: 13, padding: '24px 0', textAlign: 'center' }}>—</div>
}
