import { Download, Languages, List, Table2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { dictionaryApi } from '../api/bookflow'
import { useDebouncedValue, useDictionary, useUpdateDictionaryEntry } from '../api/hooks'
import {
  Badge,
  Button,
  DataTable,
  Drawer,
  EmptyState,
  ErrorState,
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
        render: (entry) => entry.translation
      },
      {
        key: 'language',
        header: t('dictionary.language'),
        render: (entry) =>
          `${entry.source_language.toUpperCase()} → ${entry.target_language.toUpperCase()}`
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
                <span className={styles.wordTranslation}>{entry.translation}</span>
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
  const save = async () => {
    await update.mutateAsync({ note, status })
    onClose()
  }
  return (
    <div className={styles.sidePanel} style={{ padding: 24 }}>
      <div>
        <h2 className={styles.sidePanelWord}>{entry.original_word}</h2>
        <p className={styles.sidePanelTranslation}>{entry.translation}</p>
        <p className={styles.wordTranscription}>
          {[entry.transcription, entry.part_of_speech].filter(Boolean).join(' · ')}
        </p>
      </div>
      {entry.definition ? <p>{entry.definition}</p> : null}
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
        <Button onClick={onClose}>{t('common.cancel')}</Button>
        <Button variant="accent" loading={update.isPending} onClick={() => void save()}>
          {t('common.save')}
        </Button>
      </div>
    </div>
  )
}
