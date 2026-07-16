import { Trash2 } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { clearProgressQueue } from '../api/offlineQueue'
import { useReaderPreferences, useUpdateReaderPreferences } from '../api/hooks'
import { setLocale } from '../i18n'
import { Button, Select, Switch, useToast } from '../shared/ui'
import { useOfflineStore } from '../stores/offlineStore'
import { contrastRatio } from '../reader/color'
import { useReaderStore } from '../stores/readerStore'
import { useUIStore, type AppTheme } from '../stores/uiStore'
import type { ReaderFont, ReaderTheme } from '../types/api'
import styles from './pages.module.css'

export function SettingsPage() {
  const { t, i18n } = useTranslation()
  const { notify } = useToast()
  const appTheme = useUIStore((state) => state.appTheme)
  const setAppTheme = useUIStore((state) => state.setAppTheme)
  const appThemeColors = useUIStore((state) => state.appThemeColors)
  const setAppThemeColors = useUIStore((state) => state.setAppThemeColors)
  const preferences = useReaderStore((state) => state.preferences)
  const updatePreferences = useReaderStore((state) => state.updatePreferences)
  const replacePreferences = useReaderStore((state) => state.replacePreferences)
  const query = useReaderPreferences()
  const update = useUpdateReaderPreferences()
  const pending = useOfflineStore((state) => state.pending)
  const setPending = useOfflineStore((state) => state.setPending)
  const hydrated = useRef(false)
  const appThemeContrast = contrastRatio(appThemeColors.background, appThemeColors.foreground)

  useEffect(() => {
    if (query.data && !hydrated.current) {
      hydrated.current = true
      replacePreferences(query.data)
    }
  }, [query.data, replacePreferences])

  const saveReader = async () => {
    await update.mutateAsync(preferences)
    notify(t('settings.saved'), 'success')
  }

  return (
    <div className={`${styles.page} ${styles.pageNarrow}`}>
      <header className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>{t('settings.title')}</h1>
        </div>
      </header>
      <div className={styles.settingsGrid}>
        <nav className={styles.settingsNav} aria-label={t('settings.title')}>
          <a href="#interface">{t('settings.interface')}</a>
          <a href="#reader">{t('settings.reader')}</a>
          <a href="#offline">{t('settings.offline')}</a>
        </nav>
        <div className={styles.settingsContent}>
          <section id="interface" className={styles.settingsSection}>
            <h2>{t('settings.interface')}</h2>
            <SettingRow label={t('settings.language')}>
              <Select
                aria-label={t('settings.language')}
                value={i18n.language.startsWith('ru') ? 'ru' : 'en'}
                onChange={(event) => void setLocale(event.target.value as 'ru' | 'en')}
              >
                <option value="ru">{t('common.russian')}</option>
                <option value="en">{t('common.english')}</option>
              </Select>
            </SettingRow>
            <SettingRow label={t('settings.appTheme')}>
              <Select
                aria-label={t('settings.appTheme')}
                value={appTheme}
                onChange={(event) => setAppTheme(event.target.value as AppTheme)}
              >
                <option value="system">{t('settings.system')}</option>
                <option value="light">{t('settings.light')}</option>
                <option value="warm">{t('settings.warm')}</option>
                <option value="dark">{t('settings.dark')}</option>
                <option value="custom">{t('settings.custom')}</option>
              </Select>
            </SettingRow>
            {appTheme === 'custom' ? (
              <SettingRow
                label={t('settings.customPalette')}
                description={t('settings.customPaletteDescription')}
              >
                <div className={styles.appThemePalette}>
                  <AppColorField
                    label={t('reader.background')}
                    value={appThemeColors.background}
                    onChange={(background) => setAppThemeColors({ background })}
                  />
                  <AppColorField
                    label={t('reader.textColor')}
                    value={appThemeColors.foreground}
                    onChange={(foreground) => setAppThemeColors({ foreground })}
                  />
                  <AppColorField
                    label={t('reader.accent')}
                    value={appThemeColors.accent}
                    onChange={(accent) => setAppThemeColors({ accent })}
                  />
                  {appThemeContrast < 4.5 ? (
                    <p className={styles.appThemeContrastError} role="alert">
                      {t('settings.customContrastError')}
                    </p>
                  ) : null}
                </div>
              </SettingRow>
            ) : null}
          </section>

          <section id="reader" className={styles.settingsSection}>
            <h2>{t('settings.reader')}</h2>
            <SettingRow label={t('reader.theme')}>
              <Select
                aria-label={t('reader.theme')}
                value={preferences.theme}
                onChange={(event) =>
                  updatePreferences({ theme: event.target.value as ReaderTheme })
                }
              >
                {(['light', 'warm', 'sepia', 'dark', 'custom'] as const).map((theme) => (
                  <option key={theme} value={theme}>
                    {t(`reader.${theme}`)}
                  </option>
                ))}
              </Select>
            </SettingRow>
            <SettingRow label={t('reader.font')}>
              <Select
                aria-label={t('reader.font')}
                value={preferences.font_family}
                onChange={(event) =>
                  updatePreferences({ font_family: event.target.value as ReaderFont })
                }
              >
                {(['system', 'serif', 'Georgia', 'Arial', 'Inter', 'Source Serif 4'] as const).map(
                  (font) => (
                    <option key={font} value={font}>
                      {font}
                    </option>
                  )
                )}
              </Select>
            </SettingRow>
            <SettingRow label={t('reader.size')}>
              <input
                type="range"
                min="16"
                max="30"
                value={preferences.font_size}
                aria-label={t('reader.size')}
                onChange={(event) => updatePreferences({ font_size: Number(event.target.value) })}
              />
            </SettingRow>
            <SettingRow label={t('reader.lineHeight')}>
              <input
                type="range"
                min="1.35"
                max="2"
                step="0.05"
                value={preferences.line_height}
                aria-label={t('reader.lineHeight')}
                onChange={(event) => updatePreferences({ line_height: Number(event.target.value) })}
              />
            </SettingRow>
            <SettingRow label={t('reader.width')}>
              <input
                type="range"
                min="520"
                max="960"
                step="20"
                value={preferences.content_width}
                aria-label={t('reader.width')}
                onChange={(event) =>
                  updatePreferences({ content_width: Number(event.target.value) })
                }
              />
            </SettingRow>
            <SettingRow label={t('reader.mode')}>
              <Select
                aria-label={t('reader.mode')}
                value={preferences.reading_mode}
                onChange={(event) =>
                  updatePreferences({ reading_mode: event.target.value as 'scroll' | 'paged' })
                }
              >
                <option value="scroll">{t('reader.scroll')}</option>
                <option value="paged">{t('reader.paged')}</option>
              </Select>
            </SettingRow>
            <SettingRow label={t('reader.showProgress')}>
              <Switch
                label={t('reader.showProgress')}
                checked={preferences.show_progress}
                onChange={(show_progress) => updatePreferences({ show_progress })}
              />
            </SettingRow>
            <Button variant="accent" loading={update.isPending} onClick={() => void saveReader()}>
              {t('common.save')}
            </Button>
          </section>

          <section id="offline" className={styles.settingsSection}>
            <h2>{t('settings.offline')}</h2>
            <SettingRow
              label={t('settings.offline')}
              description={`${t('settings.offlineBody')} ${t('common.queued', { count: pending })}`}
            >
              <Button
                startIcon={Trash2}
                variant="danger"
                onClick={() => void clearProgressQueue().then(() => setPending(0))}
              >
                {t('settings.clearOffline')}
              </Button>
            </SettingRow>
          </section>
        </div>
      </div>
    </div>
  )
}

function AppColorField({
  label,
  value,
  onChange
}: {
  label: string
  value: string
  onChange: (value: string) => void
}) {
  return (
    <label className={styles.appColorField}>
      <span>{label}</span>
      <span className={styles.appColorControl}>
        <input
          className={styles.appColorInput}
          type="color"
          value={value}
          aria-label={label}
          onChange={(event) => onChange(event.target.value)}
        />
        <code>{value.toUpperCase()}</code>
      </span>
    </label>
  )
}

function SettingRow({
  label,
  description,
  children
}: {
  label: string
  description?: string
  children: React.ReactNode
}) {
  return (
    <div className={styles.settingRow}>
      <div>
        <span className={styles.settingLabel}>{label}</span>
        {description ? <p className={styles.settingDescription}>{description}</p> : null}
      </div>
      <div>{children}</div>
    </div>
  )
}
