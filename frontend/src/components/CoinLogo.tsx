import type { Currency } from '../api'
import { metaOf } from '../currencies'

export function CoinLogo({ code, size = 40 }: { code: Currency; size?: number }) {
  const m = metaOf(code)
  return (
    <div
      className="coin-logo"
      style={{ background: m.color, width: size, height: size, fontSize: size * 0.46 }}
      aria-label={m.name}
    >
      {m.symbol}
    </div>
  )
}
