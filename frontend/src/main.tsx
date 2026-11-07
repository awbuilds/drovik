import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@fontsource-variable/inter'
import '@fontsource-variable/space-grotesk'
import '@fontsource-variable/jetbrains-mono'
import './styles/index.css'
import App from './app/App'
import { reportClientError } from './lib/errors'
import { initializeTheme } from './lib/theme'

window.addEventListener('error', (event) => {
  try {
    reportClientError(event.error ?? event.message, {
      operation: 'window.error',
      source: event.filename,
      line: event.lineno,
    })
  } catch {
    // Error reporting must never cause a second failure.
  }
})

window.addEventListener('unhandledrejection', (event) => {
  try {
    reportClientError(event.reason, { operation: 'window.unhandledrejection' })
  } catch {
    // Error reporting must never cause a second failure.
  }
})

try {
  initializeTheme()
  const rootElement = document.getElementById('root')
  if (!rootElement) {
    throw new Error('Root element #root is missing from index.html')
  }
  createRoot(rootElement).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
} catch (error) {
  reportClientError(error, { operation: 'bootstrap' })
  const fallback = document.createElement('div')
  fallback.style.cssText =
    'min-height:100vh;display:flex;align-items:center;justify-content:center;padding:24px;background:#07090c;color:#dfe5ec;font-family:system-ui,sans-serif;text-align:center'
  fallback.textContent =
    'Drovik could not start. Check the browser console and the Drovik service logs, then reload.'
  document.body.replaceChildren(fallback)
}
