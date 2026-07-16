import {
  BookPlus,
  Check,
  Copy,
  Highlighter,
  Languages,
  LoaderCircle,
  NotebookPen,
  Share2,
  X
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button, IconButton } from '../shared/ui'
import type { TextTranslation, WordTranslation } from '../types/api'
import styles from './reader.module.css'

export type TranslationValue = WordTranslation | TextTranslation

export function SelectionToolbar({
  position,
  onTranslate,
  onDictionary,
  onHighlight,
  onNote,
  onCopy,
  onShare
}: {
  position: { x: number; y: number }
  onTranslate: () => void
  onDictionary: () => void
  onHighlight: () => void
  onNote: () => void
  onCopy: () => void
  onShare: () => void
}) {
  const { t } = useTranslation()
  return (
    <div
      className={styles.selectionMenu}
      role="toolbar"
      aria-label={t('reader.selectHint')}
      style={{ left: position.x, top: position.y }}
    >
      <Action icon={Languages} label={t('reader.translate')} onClick={onTranslate} />
      <Action icon={BookPlus} label={t('reader.addDictionary')} onClick={onDictionary} />
      <span className={styles.selectionDivider} />
      <Action icon={Highlighter} label={t('reader.highlight')} onClick={onHighlight} />
      <Action icon={NotebookPen} label={t('reader.addNote')} onClick={onNote} />
      <Action icon={Copy} label={t('reader.copy')} onClick={onCopy} compact />
      <Action icon={Share2} label={t('reader.share')} onClick={onShare} compact />
    </div>
  )
}

function Action({
  icon: Icon,
  label,
  onClick,
  compact = false
}: {
  icon: typeof Languages
  label: string
  onClick: () => void
  compact?: boolean
}) {
  return (
    <button type="button" className={styles.selectionButton} aria-label={label} onClick={onClick}>
      <Icon size={15} aria-hidden="true" />
      {compact ? null : <span>{label}</span>}
    </button>
  )
}

export function TranslationPopover({
  selectedText,
  value,
  loading,
  error,
  position,
  added,
  mobile = false,
  onAdd,
  onClose
}: {
  selectedText: string
  value?: TranslationValue
  loading: boolean
  error: boolean
  position?: { x: number; y: number }
  added: boolean
  mobile?: boolean
  onAdd: () => void
  onClose: () => void
}) {
  const { t } = useTranslation()
  const isWord = value && 'normalized_form' in value
  const content = (
    <>
      <div className={styles.translationHeader}>
        <div>
          <h3 className={styles.translationOriginal}>{value?.original_text ?? selectedText}</h3>
          {isWord ? (
            <p className={styles.translationMeta}>
              {[value.transcription, value.part_of_speech].filter(Boolean).join(' · ')}
            </p>
          ) : null}
        </div>
        <IconButton
          className={styles.readerIcon}
          size="small"
          icon={X}
          label={t('common.close')}
          onClick={onClose}
        />
      </div>
      {loading ? (
        <div className={styles.translationLoading} role="status">
          <LoaderCircle size={18} aria-hidden="true" />
          {t('reader.translating')}
        </div>
      ) : null}
      {error ? (
        <p className={styles.contrastError} role="alert">
          {t('reader.translationError')}
        </p>
      ) : null}
      {value ? (
        <div aria-live="polite">
          <p className={styles.translationValue}>{value.translation}</p>
          {isWord && value.definition ? (
            <p className={styles.translationDefinition}>{value.definition}</p>
          ) : null}
          {isWord && value.alternatives.length ? (
            <div className={styles.translationAlternatives}>
              {value.alternatives.map((item) => (
                <span key={item} className={styles.translationTag}>
                  {item}
                </span>
              ))}
            </div>
          ) : null}
          <div className={styles.translationActions}>
            <Button
              variant="accent"
              size="small"
              startIcon={added ? Check : BookPlus}
              disabled={added || !isWord}
              onClick={onAdd}
            >
              {added ? t('reader.addedDictionary') : t('reader.addDictionary')}
            </Button>
          </div>
        </div>
      ) : null}
    </>
  )
  if (mobile) return <div className={styles.mobileSelection}>{content}</div>
  return (
    <div
      className={styles.translationPopover}
      role="dialog"
      aria-label={t('reader.translation')}
      style={{ left: position?.x ?? 12, top: position?.y ?? 60 }}
    >
      {content}
    </div>
  )
}
