import { useI18n, type Lang } from '../i18n'

export function LangToggle() {
  const { lang, setLang } = useI18n()
  const opt = (l: Lang) => (
    <button key={l} type="button" className={`lang-opt ${lang === l ? 'lang-active' : ''}`} onClick={() => setLang(l)}>
      {l.toUpperCase()}
    </button>
  )
  return <div className="lang-toggle">{opt('es')}{opt('en')}</div>
}
