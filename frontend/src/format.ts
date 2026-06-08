import type { Currency } from './api'
import { metaOf } from './currencies'

const fiatFmt: Partial<Record<Currency, Intl.NumberFormat>> = {
  CRC: new Intl.NumberFormat('es-CR', { style: 'currency', currency: 'CRC', minimumFractionDigits: 2 }),
  USD: new Intl.NumberFormat('es-CR', { style: 'currency', currency: 'USD', minimumFractionDigits: 2 }),
}

/** Format integer minor units in the given currency (fiat or crypto). */
export function formatMoney(minor: number, currency: Currency = 'CRC'): string {
  const m = metaOf(currency)
  const value = minor / 10 ** m.decimals

  const fiat = fiatFmt[currency]
  if (fiat) return fiat.format(value)

  // Crypto: trim trailing zeros, append ticker.
  const s = value.toLocaleString('en-US', { maximumFractionDigits: m.decimals })
  return `${s} ${currency}`
}

/** Backwards-compatible CRC helper. */
export function formatCents(cents: number): string {
  return formatMoney(cents, 'CRC')
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('es-CR', {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/** Pretty-print a Costa Rican 8-digit phone as 8888-0000. */
export function formatPhone(raw: string): string {
  const d = raw.replace(/\D/g, '')
  return d.length === 8 ? `${d.slice(0, 4)}-${d.slice(4)}` : raw
}
