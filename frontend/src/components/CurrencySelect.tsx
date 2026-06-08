import type { Currency } from '../api'
import { CRYPTO, FIAT } from '../currencies'

export function CurrencySelect({
  id,
  value,
  onChange,
}: {
  id?: string
  value: Currency
  onChange: (c: Currency) => void
}) {
  return (
    <select id={id} value={value} onChange={(e) => onChange(e.target.value as Currency)}>
      <optgroup label="Fiat">
        {FIAT.map((c) => (
          <option key={c.code} value={c.code}>
            {c.symbol} {c.name} ({c.code})
          </option>
        ))}
      </optgroup>
      <optgroup label="Cripto">
        {CRYPTO.map((c) => (
          <option key={c.code} value={c.code}>
            {c.symbol} {c.name} ({c.code})
          </option>
        ))}
      </optgroup>
    </select>
  )
}
