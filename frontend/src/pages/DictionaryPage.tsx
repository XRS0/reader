import { ChevronRight, Download, Languages, Plus, Trash2 } from 'lucide-react'
import { useEffect, useId, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { dictionaryApi } from '../api/bookflow'
import {
  useCreateDictionaryEntry,
  useDebouncedValue,
  useDeleteDictionaryEntry,
  useDictionary,
  useUpdateDictionaryEntry
} from '../api/hooks'
import {
  AlertDialog,
  Badge,
  Button,
  Dialog,
  Drawer,
  EmptyState,
  ErrorState,
  Field,
  IconButton,
  Input,
  SearchInput,
  Select,
  Skeleton,
  Textarea
} from '../shared/ui'
import { formatDate } from '../shared/format'
import type { DictionaryEntry, DictionaryStatus } from '../types/api'
import styles from './pages.module.css'

export function DictionaryPage() {
  const { t } = useTranslation()
  const [searchParams, setSearchParams] = useSearchParams()
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState<DictionaryStatus | 'all'>('all')
  const debounced = useDebouncedValue(search)
  const query = useDictionary({ search: debounced, status, sort: 'last_seen', limit: 80 })
  const [selected, setSelected] = useState<DictionaryEntry>()
  const [deleting, setDeleting] = useState<DictionaryEntry>()
  const [createOpen, setCreateOpen] = useState(false)
  const remove = useDeleteDictionaryEntry()

  useEffect(() => {
    const entryId = searchParams.get('entry')
    if (entryId && query.data) setSelected(query.data.items.find((item) => item.id === entryId))
  }, [query.data, searchParams])

  const openEntry = (entry: DictionaryEntry) => {
    setSelected(entry)
    setSearchParams((params) => {
      params.set('entry', entry.id)
      return params
    })
  }
  const closeEntry = () => {
    setSelected(undefined)
    setSearchParams((params) => {
      params.delete('entry')
      return params
    })
  }

  const confirmDelete = async () => {
    if (!deleting) return
    try {
      await remove.mutateAsync(deleting.id)
      if (selected?.id === deleting.id) closeEntry()
      setDeleting(undefined)
    } catch {
      // Keep the dialog open and replace its description with the mutation error.
    }
  }

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>{t('dictionary.title')}</h1>
          <p className={styles.pageSubtitle}>
            {t('dictionary.summary', { count: query.data?.total_count ?? 0 })}
          </p>
        </div>
        <div className={styles.headerActions}>
          <Button variant="accent" startIcon={Plus} onClick={() => setCreateOpen(true)}>
            {t('dictionary.addWord')}
          </Button>
          <Button
            startIcon={Download}
            onClick={() =>
              void dictionaryApi.export().then((data) => {
                const url = URL.createObjectURL(
                  new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
                )
                const anchor = document.createElement('a')
                anchor.href = url
                anchor.download = 'bookflow-dictionary.json'
                anchor.click()
                URL.revokeObjectURL(url)
              })
            }
          >
            {t('dictionary.export')}
          </Button>
        </div>
      </header>
      <div className={styles.dictionaryToolbar}>
        <SearchInput
          label={t('common.search')}
          placeholder={t('dictionary.searchPlaceholder')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          onClear={() => setSearch('')}
        />
        <Select
          className={styles.compactSelect}
          value={status}
          aria-label={t('dictionary.status')}
          onChange={(event) => setStatus(event.target.value as DictionaryStatus | 'all')}
        >
          <option value="all">{t('common.all')}</option>
          <option value="unknown">{t('dictionary.unknown')}</option>
          <option value="learning">{t('dictionary.learning')}</option>
          <option value="known">{t('dictionary.known')}</option>
          <option value="mastered">{t('dictionary.mastered')}</option>
          <option value="ignored">{t('dictionary.ignored')}</option>
        </Select>
      </div>

      {query.isLoading ? (
        <div className={styles.flatList}>
          {Array.from({ length: 8 }, (_, index) => (
            <div key={index} className={styles.flatRow}>
              <Skeleton width="55%" />
              <Skeleton width={90} />
            </div>
          ))}
        </div>
      ) : query.isError ? (
        <ErrorState
          title={t('common.errorTitle')}
          body={t('common.errorMessage')}
          retryLabel={t('common.retry')}
          onRetry={() => void query.refetch()}
        />
      ) : !query.data?.items.length ? (
        <EmptyState
          icon={Languages}
          title={t('dictionary.emptyTitle')}
          body={t('dictionary.emptyBody')}
        />
      ) : (
        <div className={styles.dictionaryList}>
          {query.data.items.map((entry) => (
            <article key={entry.id} className={styles.wordCard}>
              <button
                type="button"
                className={styles.wordCardOpen}
                aria-label={t('dictionary.openEntry', { word: entry.original_word })}
                onClick={() => openEntry(entry)}
              >
                <span className={styles.wordCardTopline}>
                  <span className={styles.wordMain}>{entry.original_word}</span>
                  <StatusBadge status={entry.status} />
                </span>
                {entry.transcription ? (
                  <span className={styles.wordTranscription}>{entry.transcription}</span>
                ) : null}
                <span className={styles.wordCardSummary}>
                  {entry.translation || entry.definition || t('dictionary.noDetails')}
                </span>
                <span className={styles.wordCardFooter}>
                  <span>
                    {entry.translation
                      ? `${entry.source_language.toUpperCase()} → ${entry.target_language.toUpperCase()}`
                      : entry.source_language.toUpperCase()}
                  </span>
                  <ChevronRight size={16} aria-hidden="true" />
                </span>
              </button>
              <IconButton
                className={styles.wordCardDelete}
                size="small"
                icon={Trash2}
                label={t('dictionary.deleteWord', { word: entry.original_word })}
                onClick={() => {
                  remove.reset()
                  setDeleting(entry)
                }}
              />
            </article>
          ))}
        </div>
      )}

      <Drawer open={Boolean(selected)} onClose={closeEntry} label={t('dictionary.entryDetails')}>
        {selected ? (
          <DictionaryDetails
            entry={selected}
            onClose={closeEntry}
            onDelete={() => setDeleting(selected)}
          />
        ) : null}
      </Drawer>
      <NewDictionaryEntryDialog open={createOpen} onClose={() => setCreateOpen(false)} />
      <AlertDialog
        open={Boolean(deleting)}
        onClose={() => {
          remove.reset()
          setDeleting(undefined)
        }}
        onConfirm={() => void confirmDelete()}
        title={deleting?.original_word ?? t('common.delete')}
        description={t(remove.isError ? 'dictionary.deleteError' : 'dictionary.deleteConfirm')}
        confirmLabel={t('common.delete')}
        cancelLabel={t('common.cancel')}
        confirmLoading={remove.isPending}
      />
    </div>
  )
}

function StatusBadge({ status }: { status: DictionaryStatus }) {
  const { t } = useTranslation()
  return (
    <Badge tone={status === 'mastered' || status === 'known' ? 'accent' : 'neutral'}>
      {t(`dictionary.${status}`)}
    </Badge>
  )
}

function DictionaryDetails({
  entry,
  onClose,
  onDelete
}: {
  entry: DictionaryEntry
  onClose: () => void
  onDelete: () => void
}) {
  const { t, i18n } = useTranslation()
  const update = useUpdateDictionaryEntry(entry.id)
  const [note, setNote] = useState(entry.note ?? '')
  const [status, setStatus] = useState(entry.status)
  const [translation, setTranslation] = useState(entry.translation)
  const [definition, setDefinition] = useState(entry.definition ?? '')
  const save = async () => {
    try {
      await update.mutateAsync({ note, status, translation, definition })
      onClose()
    } catch {
      // The mutation state renders the error without closing the drawer.
    }
  }
  return (
    <div className={styles.sidePanel} style={{ padding: 24 }}>
      <div>
        <h2 className={styles.sidePanelWord}>{entry.original_word}</h2>
        {entry.translation ? (
          <p className={styles.sidePanelTranslation}>{entry.translation}</p>
        ) : null}
        <p className={styles.wordTranscription}>
          {[entry.transcription, entry.part_of_speech].filter(Boolean).join(' · ')}
        </p>
      </div>
      {entry.definition ? (
        <div className={styles.entryDefinition}>
          <span>{t('dictionary.definition')}</span>
          <p>{entry.definition}</p>
        </div>
      ) : null}
      <Field label={t('dictionary.translation')} htmlFor={`dictionary-translation-${entry.id}`}>
        <Input
          id={`dictionary-translation-${entry.id}`}
          value={translation}
          onChange={(event) => setTranslation(event.target.value)}
        />
      </Field>
      <Field label={t('dictionary.definition')} htmlFor={`dictionary-definition-${entry.id}`}>
        <Textarea
          id={`dictionary-definition-${entry.id}`}
          value={definition}
          onChange={(event) => setDefinition(event.target.value)}
        />
      </Field>
      <label>
        <span className={styles.settingLabel}>{t('dictionary.status')}</span>
        <Select
          value={status}
          onChange={(event) => setStatus(event.target.value as DictionaryStatus)}
        >
          {(['unknown', 'learning', 'known', 'mastered', 'ignored'] as const).map((value) => (
            <option key={value} value={value}>
              {t(`dictionary.${value}`)}
            </option>
          ))}
        </Select>
      </label>
      <label>
        <span className={styles.settingLabel}>{t('dictionary.note')}</span>
        <Textarea value={note} onChange={(event) => setNote(event.target.value)} />
      </label>
      <div>
        <h3 className={styles.sectionTitle}>{t('dictionary.occurrences')}</h3>
        <div className={styles.flatRow}>
          <div>
            <p className={styles.flatTitle}>{entry.book_title ?? t('dictionary.book')}</p>
            <p className={styles.flatMeta}>
              {t('dictionary.lastSeen')}: {formatDate(entry.last_seen_at, i18n.language)}
            </p>
          </div>
          <strong>{entry.encounter_count}</strong>
        </div>
      </div>
      <div className={styles.headerActions}>
        {update.isError ? (
          <span className={styles.formError} role="alert">
            {t('dictionary.createError')}
          </span>
        ) : null}
        <Button variant="danger" startIcon={Trash2} onClick={onDelete}>
          {t('common.delete')}
        </Button>
        <Button onClick={onClose}>{t('common.cancel')}</Button>
        <Button variant="accent" loading={update.isPending} onClick={() => void save()}>
          {t('common.save')}
        </Button>
      </div>
    </div>
  )
}

function NewDictionaryEntryDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  const fieldPrefix = useId()
  const create = useCreateDictionaryEntry()
  const [word, setWord] = useState('')
  const [sourceLanguage, setSourceLanguage] = useState('ru')
  const [targetLanguage, setTargetLanguage] = useState('')
  const [translation, setTranslation] = useState('')
  const [definition, setDefinition] = useState('')
  const [validationError, setValidationError] = useState('')

  const close = () => {
    create.reset()
    setValidationError('')
    onClose()
  }
  const submit = async () => {
    if (!word.trim() || (!translation.trim() && !definition.trim())) {
      setValidationError(t('dictionary.contentRequired'))
      return
    }
    setValidationError('')
    try {
      await create.mutateAsync({
        source_language: sourceLanguage,
        target_language: translation.trim() ? targetLanguage || undefined : undefined,
        original_word: word.trim(),
        translation: translation.trim() || undefined,
        definition: definition.trim() || undefined,
        status: 'unknown'
      })
      setWord('')
      setTranslation('')
      setDefinition('')
      close()
    } catch {
      // The mutation state renders the API error in the dialog.
    }
  }

  return (
    <Dialog
      open={open}
      onClose={close}
      title={t('dictionary.addWordTitle')}
      description={t('dictionary.addWordDescription')}
      closeLabel={t('common.close')}
      footer={
        <>
          <Button onClick={close}>{t('common.cancel')}</Button>
          <Button variant="accent" loading={create.isPending} onClick={() => void submit()}>
            {t('dictionary.addWord')}
          </Button>
        </>
      }
    >
      <div className={styles.dictionaryForm}>
        <Field label={t('dictionary.word')} htmlFor={`${fieldPrefix}-word`}>
          <Input
            id={`${fieldPrefix}-word`}
            autoFocus
            value={word}
            placeholder={t('dictionary.wordPlaceholder')}
            onChange={(event) => setWord(event.target.value)}
          />
        </Field>
        <div className={styles.dictionaryLanguageFields}>
          <Field label={t('dictionary.sourceLanguage')} htmlFor={`${fieldPrefix}-source-language`}>
            <Select
              id={`${fieldPrefix}-source-language`}
              value={sourceLanguage}
              onChange={(event) => setSourceLanguage(event.target.value)}
            >
              <option value="ru">Русский</option>
              <option value="en">English</option>
            </Select>
          </Field>
          <Field label={t('dictionary.targetLanguage')} htmlFor={`${fieldPrefix}-target-language`}>
            <Select
              id={`${fieldPrefix}-target-language`}
              value={targetLanguage}
              disabled={!translation.trim()}
              onChange={(event) => setTargetLanguage(event.target.value)}
            >
              <option value="">{t('dictionary.noTranslation')}</option>
              <option value="ru">Русский</option>
              <option value="en">English</option>
            </Select>
          </Field>
        </div>
        <Field
          label={t('dictionary.translation')}
          htmlFor={`${fieldPrefix}-translation`}
          hint={t('dictionary.translationPlaceholder')}
        >
          <Input
            id={`${fieldPrefix}-translation`}
            value={translation}
            placeholder={t('dictionary.translationPlaceholder')}
            onChange={(event) => {
              const value = event.target.value
              setTranslation(value)
              if (value && !targetLanguage) setTargetLanguage(sourceLanguage === 'ru' ? 'en' : 'ru')
            }}
          />
        </Field>
        <Field
          label={t('dictionary.definition')}
          htmlFor={`${fieldPrefix}-definition`}
          error={validationError || (create.isError ? t('dictionary.createError') : undefined)}
        >
          <Textarea
            id={`${fieldPrefix}-definition`}
            value={definition}
            placeholder={t('dictionary.definitionPlaceholder')}
            onChange={(event) => setDefinition(event.target.value)}
          />
        </Field>
      </div>
    </Dialog>
  )
}
