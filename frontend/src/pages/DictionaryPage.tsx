import { Download, Languages, List, Plus, Table2 } from 'lucide-react'
import { useEffect, useId, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { dictionaryApi } from '../api/bookflow'
import {
  useCreateDictionaryEntry,
  useDebouncedValue,
  useDictionary,
  useUpdateDictionaryEntry
} from '../api/hooks'
import {
  Badge,
  Button,
  DataTable,
  Dialog,
  Drawer,
  EmptyState,
  ErrorState,
  Field,
  Input,
  SearchInput,
  Select,
  Skeleton,
  Textarea,
  type Column
} from '../shared/ui'
import { formatDate } from '../shared/format'
import type { DictionaryEntry, DictionaryStatus } from '../types/api'
import styles from './pages.module.css'

export function DictionaryPage() {
  const { t, i18n } = useTranslation()
  const [searchParams, setSearchParams] = useSearchParams()
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState<DictionaryStatus | 'all'>('all')
  const [view, setView] = useState<'table' | 'list'>('table')
  const [mobile, setMobile] = useState(() => matchMedia('(max-width: 720px)').matches)
  const debounced = useDebouncedValue(search)
  const query = useDictionary({ search: debounced, status, sort: 'last_seen', limit: 80 })
  const [selected, setSelected] = useState<DictionaryEntry>()
  const [createOpen, setCreateOpen] = useState(false)

  useEffect(() => {
    const media = matchMedia('(max-width: 720px)')
    const listener = () => setMobile(media.matches)
    media.addEventListener('change', listener)
    return () => media.removeEventListener('change', listener)
  }, [])

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

  const columns = useMemo<Column<DictionaryEntry>[]>(
    () => [
      {
        key: 'word',
        header: t('dictionary.word'),
        render: (entry) => (
          <span className={styles.wordCell}>
            <span className={styles.wordMain}>{entry.original_word}</span>
            {entry.transcription ? (
              <span className={styles.wordTranscription}>{entry.transcription}</span>
            ) : null}
          </span>
        )
      },
      {
        key: 'translation',
        header: t('dictionary.translation'),
        render: (entry) => entry.translation || '—'
      },
      {
        key: 'definition',
        header: t('dictionary.definition'),
        render: (entry) => entry.definition || '—'
      },
      {
        key: 'language',
        header: t('dictionary.language'),
        render: (entry) =>
          entry.translation
            ? `${entry.source_language.toUpperCase()} → ${entry.target_language.toUpperCase()}`
            : entry.source_language.toUpperCase()
      },
      {
        key: 'status',
        header: t('dictionary.status'),
        render: (entry) => <StatusBadge status={entry.status} />
      },
      {
        key: 'encounters',
        header: t('dictionary.encounters'),
        render: (entry) => entry.encounter_count
      },
      {
        key: 'last',
        header: t('dictionary.lastSeen'),
        render: (entry) => formatDate(entry.last_seen_at, i18n.language)
      },
      { key: 'book', header: t('dictionary.book'), render: (entry) => entry.book_title ?? '—' },
      {
        key: 'review',
        header: t('dictionary.review'),
        render: (entry) => formatDate(entry.next_review_at, i18n.language)
      }
    ],
    [i18n.language, t]
  )

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
        {!mobile ? (
          <div className={styles.viewToggle} role="group" aria-label={t('dictionary.title')}>
            <button
              type="button"
              className={clsx(styles.viewButton, view === 'table' && styles.viewButtonActive)}
              aria-label={t('dictionary.table')}
              aria-pressed={view === 'table'}
              onClick={() => setView('table')}
            >
              <Table2 size={16} />
            </button>
            <button
              type="button"
              className={clsx(styles.viewButton, view === 'list' && styles.viewButtonActive)}
              aria-label={t('dictionary.list')}
              aria-pressed={view === 'list'}
              onClick={() => setView('list')}
            >
              <List size={16} />
            </button>
          </div>
        ) : null}
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
      ) : view === 'table' && !mobile ? (
        <DataTable
          columns={columns}
          items={query.data.items}
          rowKey={(entry) => entry.id}
          label={t('dictionary.title')}
          onRowClick={openEntry}
        />
      ) : (
        <div className={styles.dictionaryList}>
          {query.data.items.map((entry) => (
            <button
              key={entry.id}
              type="button"
              className={styles.wordCard}
              onClick={() => openEntry(entry)}
            >
              <span>
                <span className={styles.wordMain}>{entry.original_word}</span>
                {entry.translation ? (
                  <span className={styles.wordTranslation}>{entry.translation}</span>
                ) : null}
                {entry.definition ? (
                  <span className={styles.wordDefinition}>{entry.definition}</span>
                ) : null}
                <span className={styles.wordMeta}>
                  <span>{entry.transcription}</span>
                  <span>{entry.book_title}</span>
                  <span>
                    {t('dictionary.encounters')}: {entry.encounter_count}
                  </span>
                </span>
              </span>
              <StatusBadge status={entry.status} />
            </button>
          ))}
        </div>
      )}

      <Drawer open={Boolean(selected)} onClose={closeEntry} label={t('dictionary.entryDetails')}>
        {selected ? <DictionaryDetails entry={selected} onClose={closeEntry} /> : null}
      </Drawer>
      <NewDictionaryEntryDialog open={createOpen} onClose={() => setCreateOpen(false)} />
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

function DictionaryDetails({ entry, onClose }: { entry: DictionaryEntry; onClose: () => void }) {
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
