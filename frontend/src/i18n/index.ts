import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import { en } from './locales/en'
import { ru } from './locales/ru'

const storedLanguage =
  typeof window === 'undefined' ? null : window.localStorage.getItem('bookflow:locale')
const browserLanguage = typeof navigator === 'undefined' ? 'ru' : navigator.language.toLowerCase()
const language =
  storedLanguage === 'en' || storedLanguage === 'ru'
    ? storedLanguage
    : browserLanguage.startsWith('ru')
      ? 'ru'
      : 'en'

void i18n.use(initReactI18next).init({
  resources: { ru: { translation: ru }, en: { translation: en } },
  lng: language,
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
  returnNull: false
})

export async function setLocale(locale: 'ru' | 'en'): Promise<void> {
  window.localStorage.setItem('bookflow:locale', locale)
  await i18n.changeLanguage(locale)
  document.documentElement.lang = locale
}

export default i18n
