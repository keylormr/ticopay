import type { Currency } from './api'

const formatters: Record<Currency, Intl.NumberFormat> = {
  CRC: new Intl.NumberFormat('es-CR', { style: 'currency', currency: 'CRC', minimumFractionDigits: 2 }),
  USD: new Intl.NumberFormat('es-CR', { style: 'currency', currency: 'USD', minimumFractionDigits: 2 }),
}

/** Format integer minor units (céntimos / cents) in the given currency. */
export function formatMoney(cents: number, currency: Currency = 'CRC'): string {
  return (formatters[currency] ?? formatters.CRC).format(cents / 100)
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

export const symbol: Record<Currency, string> = { CRC: '₡', USD: '$' }
