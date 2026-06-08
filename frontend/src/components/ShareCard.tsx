import { QRCodeSVG } from 'qrcode.react'
import { useState } from 'react'

export function ShareCard({ url, message }: { url: string; message: string }) {
  const [copied, setCopied] = useState(false)
  const wa = `https://wa.me/?text=${encodeURIComponent(`${message}\n${url}`)}`

  async function copy() {
    try {
      await navigator.clipboard.writeText(url)
      setCopied(true)
      setTimeout(() => setCopied(false), 1800)
    } catch {
      /* clipboard may be blocked; the link is still visible below */
    }
  }

  return (
    <div className="share-card">
      <div className="qr-box">
        <QRCodeSVG value={url} size={168} bgColor="#ffffff" fgColor="#002b7f" level="M" />
      </div>
      <p className="share-url">{url}</p>
      <div className="share-actions">
        <a className="btn btn-wa" href={wa} target="_blank" rel="noreferrer">
          Compartir por WhatsApp
        </a>
        <button type="button" className="btn-ghost" onClick={copy}>
          {copied ? '¡Copiado!' : 'Copiar enlace'}
        </button>
      </div>
    </div>
  )
}
