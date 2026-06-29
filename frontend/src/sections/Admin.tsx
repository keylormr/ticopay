import { useState } from 'react'
import { type Capabilities } from '../api'
import { useI18n } from '../i18n'
import { Reportes } from './admin/Reportes'
import { Usuarios } from './admin/Usuarios'
import { ComerciosAdmin } from './admin/ComerciosAdmin'

type Sub = 'reportes' | 'usuarios' | 'comercios'

// Admin is the back-office container. Sub-tabs are shown per capability; the
// server still enforces every endpoint, so hiding a tab is UX only.
export function Admin({ caps }: { caps: Capabilities }) {
  const { t } = useI18n()
  const tabs = (
    [
      { id: 'reportes', show: caps.reports },
      { id: 'usuarios', show: caps.backoffice },
      { id: 'comercios', show: caps.backoffice },
    ] as { id: Sub; show: boolean }[]
  ).filter((x) => x.show)

  const [sub, setSub] = useState<Sub>(tabs[0]?.id ?? 'reportes')

  return (
    <div>
      <div className="tabs" style={{ marginBottom: 14 }}>
        {tabs.map((tb) => (
          <button key={tb.id} className={`tab ${sub === tb.id ? 'tab-active' : ''}`} onClick={() => setSub(tb.id)}>
            {t(`admin.tab.${tb.id}`)}
          </button>
        ))}
      </div>
      {sub === 'reportes' && caps.reports && <Reportes />}
      {sub === 'usuarios' && caps.backoffice && <Usuarios caps={caps} />}
      {sub === 'comercios' && caps.backoffice && <ComerciosAdmin caps={caps} />}
    </div>
  )
}
