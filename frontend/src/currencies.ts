import type { Currency } from './api'

export interface CurrencyMeta {
  code: Currency
  type: 'fiat' | 'crypto'
  decimals: number
  symbol: string
  name: string
}

export const CURRENCIES: CurrencyMeta[] = [
  // Fiat
  { code: 'CRC', type: 'fiat', decimals: 2, symbol: '₡', name: 'Colón' },
  { code: 'USD', type: 'fiat', decimals: 2, symbol: '$', name: 'Dólar' },
  // Crypto
  { code: 'BTC', type: 'crypto', decimals: 8, symbol: '₿', name: 'Bitcoin' },
  { code: 'ETH', type: 'crypto', decimals: 8, symbol: 'Ξ', name: 'Ethereum' },
  { code: 'USDT', type: 'crypto', decimals: 2, symbol: '₮', name: 'Tether USD' },
  { code: 'USDC', type: 'crypto', decimals: 2, symbol: '$', name: 'USD Coin' },
  { code: 'BNB', type: 'crypto', decimals: 8, symbol: 'BNB', name: 'BNB' },
  { code: 'SOL', type: 'crypto', decimals: 8, symbol: '◎', name: 'Solana' },
  { code: 'XRP', type: 'crypto', decimals: 6, symbol: 'XRP', name: 'XRP' },
  { code: 'ADA', type: 'crypto', decimals: 6, symbol: '₳', name: 'Cardano' },
  { code: 'DOGE', type: 'crypto', decimals: 8, symbol: 'Ð', name: 'Dogecoin' },
  { code: 'TRX', type: 'crypto', decimals: 6, symbol: 'TRX', name: 'TRON' },
  { code: 'DOT', type: 'crypto', decimals: 8, symbol: 'DOT', name: 'Polkadot' },
  { code: 'LTC', type: 'crypto', decimals: 8, symbol: 'Ł', name: 'Litecoin' },
  { code: 'LINK', type: 'crypto', decimals: 8, symbol: 'LINK', name: 'Chainlink' },
  { code: 'AVAX', type: 'crypto', decimals: 8, symbol: 'AVAX', name: 'Avalanche' },
  { code: 'MATIC', type: 'crypto', decimals: 8, symbol: 'MATIC', name: 'Polygon' },
]

export const FIAT = CURRENCIES.filter((c) => c.type === 'fiat')
export const CRYPTO = CURRENCIES.filter((c) => c.type === 'crypto')

export function metaOf(code: Currency): CurrencyMeta {
  return CURRENCIES.find((c) => c.code === code) ?? CURRENCIES[0]
}

export function isCrypto(code: Currency): boolean {
  return metaOf(code).type === 'crypto'
}
