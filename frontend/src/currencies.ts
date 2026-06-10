import type { Currency } from './api'

export interface CurrencyMeta {
  code: Currency
  type: 'fiat' | 'crypto'
  decimals: number
  symbol: string
  name: string
  color: string
}

export const CURRENCIES: CurrencyMeta[] = [
  // Fiat
  { code: 'CRC', type: 'fiat', decimals: 2, symbol: '₡', name: 'Colón', color: '#002b7f' },
  { code: 'USD', type: 'fiat', decimals: 2, symbol: '$', name: 'Dólar', color: '#15803d' },
  { code: 'EUR', type: 'fiat', decimals: 2, symbol: '€', name: 'Euro', color: '#0e4c92' },
  { code: 'MXN', type: 'fiat', decimals: 2, symbol: 'MX$', name: 'Peso mexicano', color: '#006847' },
  // Crypto
  { code: 'BTC', type: 'crypto', decimals: 8, symbol: '₿', name: 'Bitcoin', color: '#f7931a' },
  { code: 'ETH', type: 'crypto', decimals: 8, symbol: 'Ξ', name: 'Ethereum', color: '#627eea' },
  { code: 'USDT', type: 'crypto', decimals: 2, symbol: '₮', name: 'Tether', color: '#26a17b' },
  { code: 'USDC', type: 'crypto', decimals: 2, symbol: '$', name: 'USD Coin', color: '#2775ca' },
  { code: 'BNB', type: 'crypto', decimals: 8, symbol: 'B', name: 'BNB', color: '#f3ba2f' },
  { code: 'SOL', type: 'crypto', decimals: 8, symbol: '◎', name: 'Solana', color: '#9945ff' },
  { code: 'XRP', type: 'crypto', decimals: 6, symbol: 'X', name: 'XRP', color: '#23292f' },
  { code: 'ADA', type: 'crypto', decimals: 6, symbol: '₳', name: 'Cardano', color: '#0033ad' },
  { code: 'DOGE', type: 'crypto', decimals: 8, symbol: 'Ð', name: 'Dogecoin', color: '#c2a633' },
  { code: 'TRX', type: 'crypto', decimals: 6, symbol: 'T', name: 'TRON', color: '#eb0029' },
  { code: 'DOT', type: 'crypto', decimals: 8, symbol: '●', name: 'Polkadot', color: '#e6007a' },
  { code: 'LTC', type: 'crypto', decimals: 8, symbol: 'Ł', name: 'Litecoin', color: '#345d9d' },
  { code: 'LINK', type: 'crypto', decimals: 8, symbol: '⬡', name: 'Chainlink', color: '#2a5ada' },
  { code: 'AVAX', type: 'crypto', decimals: 8, symbol: 'A', name: 'Avalanche', color: '#e84142' },
  { code: 'MATIC', type: 'crypto', decimals: 8, symbol: '⬣', name: 'Polygon', color: '#8247e5' },
]

export const FIAT = CURRENCIES.filter((c) => c.type === 'fiat')
export const CRYPTO = CURRENCIES.filter((c) => c.type === 'crypto')

export function metaOf(code: Currency): CurrencyMeta {
  return CURRENCIES.find((c) => c.code === code) ?? CURRENCIES[0]
}

export function isCrypto(code: Currency): boolean {
  return metaOf(code).type === 'crypto'
}
