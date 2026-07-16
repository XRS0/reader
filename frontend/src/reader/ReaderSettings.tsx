import { Check, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { IconButton, Select, Switch } from '../shared/ui'
import { contrastRatio } from './color'
import type { ReaderFont, ReaderPreferences, ReaderTheme } from '../types/api'
import styles from './reader.module.css'

const themes: Array<{ id: ReaderTheme; background: string; foreground: string }> = [
  { id: 'light', background: '#fbfbfa', foreground: '#262624' },
  { id: 'warm', background: '#f8f1df', foreground: '#302d27' },
  { id: 'sepia', background: '#eadcc2', foreground: '#3d3226' },
  { id: 'dark', background: '#242422', foreground: '#d8d8d2' },
  {
    id: 'custom',
    background: 'linear-gradient(135deg,#eef2ee 50%,#383b39 50%)',
    foreground: '#ffffff'
  }
]

const fonts: Array<{ value: ReaderFont; label: string; css: string }> = [
  { value: 'system', label: 'System sans', css: 'var(--font-reading-sans)' },
  { value: 'serif', label: 'Neutral serif', css: 'ui-serif, Georgia, serif' },
  { value: 'Georgia', label: 'Georgia', css: 'Georgia, serif' },
  { value: 'Arial', label: 'Arial', css: 'Arial, sans-serif' },
  { value: 'Inter', label: 'Inter', css: 'Inter, sans-serif' },
  { value: 'Source Serif 4', label: 'Source Serif', css: '"Source Serif 4", Georgia, serif' }
]

export function readerFontCss(font: ReaderFont): string {
  return fonts.find((item) => item.value === font)?.css ?? 'Georgia, serif'
}

export function ReaderSettings({
  preferences,
  onChange,
  onClose
}: {
  preferences: ReaderPreferences
  onChange: (input: Partial<ReaderPreferences>) => void
  onClose: () => void
}) {
  const { t } = useTranslation()
  const customContrast = contrastRatio(preferences.background_color, preferences.text_color)
  return (
    <aside
      className={clsx(styles.panel, styles.panelRight)}
      role="dialog"
      aria-modal="true"
      aria-label={t('reader.appearance')}
      onKeyDown={(event) => {
        if (event.key === 'Escape') onClose()
      }}
    >
      <div className={styles.panelHeader}>
        <h2 className={styles.panelTitle}>{t('reader.appearance')}</h2>
        <IconButton
          autoFocus
          className={styles.readerIcon}
          size="small"
          icon={X}
          label={t('common.close')}
          onClick={onClose}
        />
      </div>
      <div className={styles.settings}>
        <section className={styles.settingsGroup}>
          <h3 className={styles.settingsLabel}>{t('reader.theme')}</h3>
          <div className={styles.themeGrid}>
            {themes.map((theme) => (
              <button
                key={theme.id}
                type="button"
                className={styles.themeButton}
                aria-pressed={preferences.theme === theme.id}
                onClick={() => onChange({ theme: theme.id })}
              >
                <span
                  className={styles.themeSwatch}
                  style={{ background: theme.background, color: theme.foreground }}
                >
                  {preferences.theme === theme.id ? <Check size={14} aria-hidden="true" /> : null}
                </span>
                <span>{t(`reader.${theme.id}`)}</span>
              </button>
            ))}
          </div>
          {preferences.theme === 'custom' ? (
            <>
              <div className={styles.colorRow}>
                <ColorField
                  label={t('reader.background')}
                  value={preferences.background_color}
                  onChange={(background_color) => onChange({ background_color })}
                />
                <ColorField
                  label={t('reader.textColor')}
                  value={preferences.text_color}
                  onChange={(text_color) => onChange({ text_color })}
                />
                <ColorField
                  label={t('reader.accent')}
                  value={preferences.accent_color}
                  onChange={(accent_color) => onChange({ accent_color })}
                />
              </div>
              {customContrast < 4.5 ? (
                <p className={styles.contrastError} role="alert">
                  {t('reader.customContrastError')}
                </p>
              ) : null}
            </>
          ) : null}
        </section>

        <section className={styles.settingsGroup}>
          <h3 className={styles.settingsLabel}>{t('reader.font')}</h3>
          <Select
            value={preferences.font_family}
            onChange={(event) => onChange({ font_family: event.target.value as ReaderFont })}
            aria-label={t('reader.font')}
          >
            {fonts.map((font) => (
              <option key={font.value} value={font.value}>
                {font.label}
              </option>
            ))}
          </Select>
          <Range
            label={t('reader.size')}
            value={preferences.font_size}
            min={16}
            max={30}
            step={1}
            suffix="px"
            onChange={(font_size) => onChange({ font_size })}
          />
          <Range
            label={t('reader.lineHeight')}
            value={preferences.line_height}
            min={1.35}
            max={2}
            step={0.05}
            onChange={(line_height) => onChange({ line_height })}
          />
          <Range
            label={t('reader.letterSpacing')}
            value={preferences.letter_spacing}
            min={-0.02}
            max={0.08}
            step={0.01}
            suffix="em"
            onChange={(letter_spacing) => onChange({ letter_spacing })}
          />
        </section>

        <section className={styles.settingsGroup}>
          <h3 className={styles.settingsLabel}>{t('reader.width')}</h3>
          <Range
            label={t('reader.width')}
            value={preferences.content_width}
            min={520}
            max={960}
            step={20}
            suffix="px"
            onChange={(content_width) => onChange({ content_width })}
          />
          <Range
            label={t('reader.margins')}
            value={preferences.page_margin}
            min={16}
            max={96}
            step={8}
            suffix="px"
            onChange={(page_margin) => onChange({ page_margin })}
          />
          <div className={styles.segmented} role="group" aria-label={t('reader.align')}>
            {(['left', 'justify'] as const).map((align) => (
              <button
                key={align}
                type="button"
                className={clsx(
                  styles.segment,
                  preferences.text_align === align && styles.segmentActive
                )}
                aria-pressed={preferences.text_align === align}
                onClick={() => onChange({ text_align: align })}
              >
                {t(`reader.${align}`)}
              </button>
            ))}
          </div>
        </section>

        <section className={styles.settingsGroup}>
          <h3 className={styles.settingsLabel}>{t('reader.mode')}</h3>
          <div className={styles.segmented} role="group" aria-label={t('reader.mode')}>
            {(['scroll', 'paged'] as const).map((mode) => (
              <button
                key={mode}
                type="button"
                className={clsx(
                  styles.segment,
                  preferences.reading_mode === mode && styles.segmentActive
                )}
                aria-pressed={preferences.reading_mode === mode}
                onClick={() => onChange({ reading_mode: mode })}
              >
                {t(`reader.${mode}`)}
              </button>
            ))}
          </div>
          <Switch
            label={t('reader.showProgress')}
            checked={preferences.show_progress}
            onChange={(show_progress) => onChange({ show_progress })}
          />
          <Switch
            label={t('reader.showRemaining')}
            checked={preferences.show_remaining_time}
            onChange={(show_remaining_time) => onChange({ show_remaining_time })}
          />
          <Range
            label={t('reader.brightness')}
            value={preferences.controls_brightness}
            min={0.45}
            max={1}
            step={0.05}
            onChange={(controls_brightness) => onChange({ controls_brightness })}
          />
        </section>
      </div>
    </aside>
  )
}

function Range({
  label,
  value,
  min,
  max,
  step,
  suffix = '',
  onChange
}: {
  label: string
  value: number
  min: number
  max: number
  step: number
  suffix?: string
  onChange: (value: number) => void
}) {
  return (
    <label className={styles.settingsGroup}>
      <span className={styles.settingsLabel} style={{ textTransform: 'none', letterSpacing: 0 }}>
        {label}
      </span>
      <span className={styles.rangeRow}>
        <input
          className={styles.range}
          type="range"
          min={min}
          max={max}
          step={step}
          value={value}
          aria-label={label}
          onChange={(event) => onChange(Number(event.target.value))}
        />
        <output className={styles.rangeValue}>
          {Number(value.toFixed(2))}
          {suffix}
        </output>
      </span>
    </label>
  )
}
function ColorField({
  label,
  value,
  onChange
}: {
  label: string
  value: string
  onChange: (value: string) => void
}) {
  return (
    <label className={styles.colorField}>
      {label}
      <input
        className={styles.colorInput}
        type="color"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  )
}
