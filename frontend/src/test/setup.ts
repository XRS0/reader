import '@testing-library/jest-dom/vitest'
import 'fake-indexeddb/auto'

const memory = new Map<string, string>()
const storage: Storage = {
  get length() {
    return memory.size
  },
  clear: () => memory.clear(),
  getItem: (key) => memory.get(key) ?? null,
  key: (index) => [...memory.keys()][index] ?? null,
  removeItem: (key) => memory.delete(key),
  setItem: (key, value) => memory.set(key, String(value))
}

Object.defineProperty(window, 'localStorage', {
  configurable: true,
  value: storage
})
Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: storage
})

await import('../i18n')

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false
  })
})

Object.defineProperty(navigator, 'onLine', { configurable: true, value: true })
if (!globalThis.crypto.randomUUID) {
  Object.defineProperty(globalThis.crypto, 'randomUUID', {
    value: () => '00000000-0000-4000-8000-000000000000'
  })
}

beforeEach(() => {
  document.body.innerHTML = '<div id="root"></div><div id="portal-root"></div>'
  localStorage.clear()
})
