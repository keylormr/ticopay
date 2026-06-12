import { describe, expect, it } from 'vitest'
import { formatMoney, formatPhone } from './format'
import { CRYPTO, FIAT, isCrypto, metaOf } from './currencies'

describe('formatMoney', () => {
  it('formats fiat from minor units', () => {
    // es-CR uses non-breaking spaces and locale separators; assert on the
    // digits/symbol rather than exact whitespace.
    expect(formatMoney(500000, 'CRC')).toContain('5')
    expect(formatMoney(150, 'USD')).toMatch(/1[.,]50/)
    expect(formatMoney(100, 'EUR')).toMatch(/1[.,]00/)
    expect(formatMoney(9999, 'MXN')).toMatch(/99[.,]99/)
  })

  it('formats crypto with ticker and trimmed zeros', () => {
    expect(formatMoney(100000000, 'BTC')).toBe('1 BTC')
    expect(formatMoney(1, 'BTC')).toBe('0.00000001 BTC')
    expect(formatMoney(2500000, 'XRP')).toBe('2.5 XRP')
  })
})

describe('formatPhone', () => {
  it('hyphenates 8-digit CR numbers', () => {
    expect(formatPhone('88880000')).toBe('8888-0000')
    expect(formatPhone('8888-0000')).toBe('8888-0000')
  })
  it('leaves anything else untouched', () => {
    expect(formatPhone('123')).toBe('123')
    expect(formatPhone('+50688880000')).toBe('+50688880000')
  })
})

describe('currencies catalog', () => {
  it('splits fiat and crypto correctly', () => {
    expect(FIAT.map((c) => c.code)).toEqual(['CRC', 'USD', 'EUR', 'MXN'])
    expect(CRYPTO.length).toBe(15)
  })

  it('metaOf falls back to CRC for unknown codes', () => {
    // @ts-expect-error deliberately invalid code
    expect(metaOf('XXX').code).toBe('CRC')
  })

  it('isCrypto distinguishes types', () => {
    expect(isCrypto('BTC')).toBe(true)
    expect(isCrypto('CRC')).toBe(false)
    expect(isCrypto('MXN')).toBe(false)
  })

  it('every currency has sane decimals', () => {
    for (const c of [...FIAT, ...CRYPTO]) {
      expect(c.decimals).toBeGreaterThanOrEqual(0)
      expect(c.decimals).toBeLessThanOrEqual(8)
    }
  })
})
