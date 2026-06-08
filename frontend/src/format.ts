const crc = new Intl.NumberFormat('es-CR', {
  style: 'currency',
  currency: 'CRC',
  minimumFractionDigits: 2,
})

/** Format integer céntimos as Costa Rican colones, e.g. 25000000 -> "₡250 000,00". */
export function formatCents(cents: number): string {
  return crc.format(cents / 100)
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('es-CR', {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}
