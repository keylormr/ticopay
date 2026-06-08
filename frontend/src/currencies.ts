import type { Currency } from './api'

export interface CurrencyMeta {
  code: Currency
  type: 'fiat' | 'crypto'
  decimals: number
  symbol: string
  name: string
}

export const CURRENCIES: CurrencyMeta[] = [
  { code: 'CRC', type: 'fiat', decimals: 2, symbol: '₡', name: 'Colón' },
  { code: 'USD', type: 'fiat', decimals: 2, symbol: '$', name: 'Dólar' },
  { code: 'BTC', type: 'crypto', decimals: 8, symbol: '₿', name: 'Bitcoin' },
  { code: 'ETH', type: 'crypto', decimals: 8, symbol: 'Ξ', name: 'Ethereum' },
  { code: 'USDT', type: 'crypto', decimals: 2, symbol: '₮', name: 'Tether USD' },
]

export const FIAT = CURRENCIES.filter((c) => c.type === 'fiat')
export const CRYPTO = CURRENCIES.filter((c) => c.type === 'crypto')

export function metaOf(code: Currency): CurrencyMeta {
  return CURRENCIES.find((c) => c.code === code) ?? CURRENCIES[0]
}

export function isCrypto(code: Currency): boolean {
  return metaOf(code).type === 'crypto'
}
