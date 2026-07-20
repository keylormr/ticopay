import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { AuthProvider } from './auth'
import { RatesProvider } from './rates'
import { I18nProvider } from './i18n'
import { warmBackend } from './api'
import './styles.css'

// Wake the (possibly cold) backend as early as possible — before React even
// mounts — so it spins up in parallel with the app loading and the user typing.
warmBackend()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <I18nProvider>
      <BrowserRouter>
        <AuthProvider>
          <RatesProvider>
            <App />
          </RatesProvider>
        </AuthProvider>
      </BrowserRouter>
    </I18nProvider>
  </StrictMode>,
)
