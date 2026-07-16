import { useEffect, useMemo, useState } from 'react'
import { FileUp, Grid2X2, List, Plus, Table2 } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { BookCard, BookCover } from '../entities/BookCard'
import {
  useBooks,
  useDebouncedValue,
  useMutateBook,
  useRemoveBook,
  useReprocessBook,
  useUploadBook,
  useCurrentUser
} from '../api/hooks'
import {
  AlertDialog,
  Button,
  DataTable,
  Dialog,
  EmptyState,
  ErrorState,
  Field,
  Input,
  ProgressBar,
  SearchInput,
  Select,
  Skeleton,
  Textarea,
  useToast,
  type Column
} from '../shared/ui'
import { formatDate } from '../shared/format'
import { useUIStore, type LibraryView } from '../stores/uiStore'
import type { Book, BookFormat, ProcessingStatus } from '../types/api'
import styles from './pages.module.css'

function librarySortFromParams(
  params: URLSearchParams
): 'last_read' | 'added' | 'title' | 'progress' {
  const value = params.get('sort')
  return value === 'last_read' || value === 'title' || value === 'progress' ? value : 'added'
}

export function LibraryPage() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { notify } = useToast()
  const user = useCurrentUser().data?.user
  const view = useUIStore((state) => state.libraryView)
  const setView = useUIStore((state) => state.setLibraryView)
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState<ProcessingStatus | 'all'>('all')
  const [format, setFormat] = useState<BookFormat | 'all'>('all')
  const [sort, setSort] = useState<'last_read' | 'added' | 'title' | 'progress'>(
    librarySortFromParams(searchParams)
  )
  useEffect(() => {
    setSort(librarySortFromParams(searchParams))
  }, [searchParams])
  const [uploadOpen, setUploadOpen] = useState(false)
  const [selectedFile, setSelectedFile] = useState<File>()
  const [editingBook, setEditingBook] = useState<Book>()
  const [deletingBook, setDeletingBook] = useState<Book>()
  const debouncedSearch = useDebouncedValue(search)
  const favorite = searchParams.get('favorite') === 'true' ? true : undefined
  const onlyContinue = searchParams.get('filter') === 'continue'
  const query = useBooks({ search: debouncedSearch, status, format, sort, favorite, limit: 60 })
  const upload = useUploadBook()
  const mutateBook = useMutateBook()
  const removeBook = useRemoveBook()
  const reprocess = useReprocessBook()
  const allBooks = query.data?.items ?? []
  const books = onlyContinue ? allBooks.filter((book) => book.progress_percent > 0) : allBooks
  const continueBook = [...allBooks]
    .filter((book) => book.processing_status === 'ready' && book.progress_percent > 0)
    .sort((a, b) => (b.last_read_at ?? '').localeCompare(a.last_read_at ?? ''))[0]

  const submitUpload = async () => {
    if (!selectedFile) return
    const extension = selectedFile.name.toLowerCase().split('.').pop()
    if (
      !['epub', 'fb2', 'txt'].includes(extension ?? '') ||
      selectedFile.size > 100 * 1024 * 1024
    ) {
      notify(t('library.uploadHint'), 'error')
      return
    }
    await upload.mutateAsync(selectedFile)
    notify(t('library.queued'), 'success')
    setUploadOpen(false)
    setSelectedFile(undefined)
  }

  const tableColumns = useMemo<Column<Book>[]>(
    () => [
      {
        key: 'book',
        header: t('library.tableBook'),
        render: (book) => (
          <div className={styles.tableBookCell}>
            <BookCover book={book} className={styles.tableCover} />
            <span>
              <span className={styles.tableTitle}>{book.title}</span>
              <span className={styles.tableAuthor}>{book.author}</span>
            </span>
          </div>
        )
      },
      {
        key: 'format',
        header: t('library.tableFormat'),
        render: (book) => book.format.toUpperCase()
      },
      {
        key: 'language',
        header: t('library.tableLanguage'),
        render: (book) => book.language.toUpperCase()
      },
      {
        key: 'progress',
        header: t('library.tableProgress'),
        render: (book) => (
          <div className={styles.tableProgress}>
            <ProgressBar
              value={book.progress_percent}
              label={t('library.progress', { value: Math.round(book.progress_percent) })}
            />
            <span>{Math.round(book.progress_percent)}%</span>
          </div>
        )
      },
      {
        key: 'last',
        header: t('library.tableLastRead'),
        render: (book) =>
          book.last_read_at ? formatDate(book.last_read_at, i18n.language) : t('library.neverRead')
      }
    ],
    [i18n.language, t]
  )

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>{t('library.title')}</h1>
          <p className={styles.pageSubtitle}>
            {t('library.greeting', { name: user?.display_name ?? '' })}
            {query.data
              ? ` ${t('library.summary', { count: query.data.total_count ?? books.length, minutes: continueBook?.estimated_minutes_remaining ?? 0 })}`
              : ''}
          </p>
        </div>
        <div className={styles.headerActions}>
          <Button variant="accent" startIcon={Plus} onClick={() => setUploadOpen(true)}>
            {t('library.upload')}
          </Button>
        </div>
      </header>

      <div className={styles.libraryToolbar}>
        <SearchInput
          label={t('common.search')}
          placeholder={t('library.searchPlaceholder')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          onClear={() => setSearch('')}
        />
        <div className={styles.filterGroup} aria-label={t('library.filters')}>
          <Select
            className={styles.compactSelect}
            aria-label={t('library.status')}
            value={status}
            onChange={(event) => setStatus(event.target.value as ProcessingStatus | 'all')}
          >
            <option value="all">{t('common.all')}</option>
            <option value="ready">{t('library.ready')}</option>
            <option value="processing">{t('library.processing')}</option>
            <option value="failed">{t('library.failed')}</option>
          </Select>
          <Select
            className={styles.compactSelect}
            aria-label={t('library.format')}
            value={format}
            onChange={(event) => setFormat(event.target.value as BookFormat | 'all')}
          >
            <option value="all">{t('common.all')}</option>
            <option value="epub">EPUB</option>
            <option value="fb2">FB2</option>
            <option value="txt">TXT</option>
          </Select>
          <Select
            className={styles.compactSelect}
            aria-label={t('library.sort')}
            value={sort}
            onChange={(event) => {
              const nextSort = event.target.value as typeof sort
              setSort(nextSort)
              const nextParams = new URLSearchParams(searchParams)
              if (nextSort === 'added') nextParams.delete('sort')
              else nextParams.set('sort', nextSort)
              setSearchParams(nextParams, { replace: true })
            }}
          >
            <option value="added">{t('library.recentlyAdded')}</option>
            <option value="last_read">{t('library.lastRead')}</option>
            <option value="title">{t('library.titleSort')}</option>
            <option value="progress">{t('library.progressSort')}</option>
          </Select>
        </div>
        <ViewToggle value={view} onChange={setView} />
      </div>

      {continueBook && !search && status === 'all' && format === 'all' ? (
        <section aria-labelledby="continue-heading">
          <div className={styles.sectionHeader}>
            <h2 id="continue-heading" className={styles.sectionTitle}>
              {t('library.continueReading')}
            </h2>
          </div>
          <div className={styles.continueStrip}>
            <BookCover book={continueBook} className={styles.continueCover} />
            <div>
              <h3 className={styles.continueTitle}>{continueBook.title}</h3>
              <p className={styles.continueAuthor}>{continueBook.author}</p>
              <ProgressBar
                value={continueBook.progress_percent}
                label={t('library.progress', { value: Math.round(continueBook.progress_percent) })}
              />
              <div className={styles.continueMeta}>
                <span>
                  {t('library.progress', { value: Math.round(continueBook.progress_percent) })}
                </span>
                <span>
                  {t('library.remaining', { count: continueBook.estimated_minutes_remaining ?? 0 })}
                </span>
              </div>
            </div>
            <Button variant="accent" onClick={() => navigate(`/read/${continueBook.id}`)}>
              {t('book.read')}
            </Button>
          </div>
        </section>
      ) : null}

      <section className={styles.section} aria-labelledby="books-heading">
        <div className={styles.sectionHeader}>
          <h2 id="books-heading" className={styles.sectionTitle}>
            {onlyContinue ? t('library.continueReading') : t('library.recentlyAdded')}
          </h2>
          {query.data ? (
            <span className={styles.sectionMeta}>{query.data.total_count ?? books.length}</span>
          ) : null}
        </div>
        {query.isLoading ? (
          <LibrarySkeleton view={view} />
        ) : query.isError ? (
          <ErrorState
            title={t('common.errorTitle')}
            body={t('common.errorMessage')}
            retryLabel={t('common.retry')}
            onRetry={() => void query.refetch()}
          />
        ) : books.length === 0 ? (
          <EmptyState
            icon={FileUp}
            title={
              search || status !== 'all' || format !== 'all'
                ? t('library.filteredEmptyTitle')
                : t('library.emptyTitle')
            }
            body={
              search || status !== 'all' || format !== 'all'
                ? t('library.filteredEmptyBody')
                : t('library.emptyBody')
            }
            action={
              !search && status === 'all' && format === 'all' ? (
                <Button startIcon={Plus} onClick={() => setUploadOpen(true)}>
                  {t('library.upload')}
                </Button>
              ) : undefined
            }
          />
        ) : view === 'table' ? (
          <DataTable
            columns={tableColumns}
            items={books}
            rowKey={(book) => book.id}
            label={t('library.title')}
            onRowClick={(book) => navigate(`/books/${book.id}`)}
          />
        ) : (
          <div className={view === 'grid' ? styles.bookGrid : styles.bookList}>
            {books.map((book) => (
              <BookCard
                key={book.id}
                book={book}
                view={view}
                onFavorite={(value) =>
                  void mutateBook.mutateAsync({
                    bookId: value.id,
                    input: { is_favorite: !value.is_favorite }
                  })
                }
                onEdit={setEditingBook}
                onDelete={setDeletingBook}
                onReprocess={(value) => void reprocess.mutateAsync(value.id)}
              />
            ))}
          </div>
        )}
      </section>

      <Dialog
        open={uploadOpen}
        onClose={() => setUploadOpen(false)}
        title={t('library.uploadTitle')}
        closeLabel={t('common.close')}
        description={t('library.uploadHint')}
        footer={
          <>
            <Button onClick={() => setUploadOpen(false)}>{t('common.cancel')}</Button>
            <Button
              variant="accent"
              loading={upload.isPending}
              disabled={!selectedFile}
              onClick={() => void submitUpload()}
            >
              {t('library.upload')}
            </Button>
          </>
        }
      >
        <label className={styles.uploadDrop}>
          <FileUp size={28} aria-hidden="true" />
          <span className={styles.fileName}>{selectedFile?.name ?? t('library.chooseFile')}</span>
          <span className={styles.sectionMeta}>{t('library.uploadHint')}</span>
          <input
            className="sr-only"
            type="file"
            accept=".epub,.fb2,.txt,application/epub+zip,text/plain,application/xml"
            onChange={(event) => setSelectedFile(event.target.files?.[0])}
          />
        </label>
      </Dialog>

      {editingBook ? (
        <MetadataDialog
          book={editingBook}
          onClose={() => setEditingBook(undefined)}
          onSave={(input) => {
            void mutateBook
              .mutateAsync({ bookId: editingBook.id, input })
              .then(() => setEditingBook(undefined))
          }}
          loading={mutateBook.isPending}
        />
      ) : null}

      <AlertDialog
        open={Boolean(deletingBook)}
        onClose={() => setDeletingBook(undefined)}
        onConfirm={() => {
          if (!deletingBook) return
          void removeBook.mutateAsync(deletingBook.id).then(() => setDeletingBook(undefined))
        }}
        title={deletingBook?.title ?? t('common.delete')}
        description={t('library.removeConfirm')}
        confirmLabel={t('common.delete')}
        cancelLabel={t('common.cancel')}
      />
    </div>
  )
}

function ViewToggle({
  value,
  onChange
}: {
  value: LibraryView
  onChange: (view: LibraryView) => void
}) {
  const { t } = useTranslation()
  const options = [
    { value: 'grid' as const, label: t('library.grid'), icon: Grid2X2 },
    { value: 'list' as const, label: t('library.list'), icon: List },
    { value: 'table' as const, label: t('library.table'), icon: Table2 }
  ]
  return (
    <div className={styles.viewToggle} role="group" aria-label={t('library.title')}>
      {options.map((option) => {
        const Icon = option.icon
        return (
          <button
            key={option.value}
            type="button"
            className={clsx(styles.viewButton, value === option.value && styles.viewButtonActive)}
            aria-label={option.label}
            aria-pressed={value === option.value}
            onClick={() => onChange(option.value)}
          >
            <Icon size={16} aria-hidden="true" />
          </button>
        )
      })}
    </div>
  )
}

function LibrarySkeleton({ view }: { view: LibraryView }) {
  if (view === 'list' || view === 'table') {
    return (
      <div className={styles.bookList}>
        {Array.from({ length: 6 }, (_, index) => (
          <div key={index} className={styles.flatRow}>
            <Skeleton height={54} />
            <Skeleton width={80} />
          </div>
        ))}
      </div>
    )
  }
  return (
    <div className={styles.bookGrid}>
      {Array.from({ length: 12 }, (_, index) => (
        <div key={index}>
          <Skeleton height={220} />
          <div style={{ height: 12 }} />
          <Skeleton width="78%" />
          <div style={{ height: 8 }} />
          <Skeleton width="52%" height={12} />
        </div>
      ))}
    </div>
  )
}

function MetadataDialog({
  book,
  onClose,
  onSave,
  loading
}: {
  book: Book
  onClose: () => void
  onSave: (input: Pick<Book, 'title' | 'author' | 'description' | 'language'>) => void
  loading: boolean
}) {
  const { t } = useTranslation()
  const [title, setTitle] = useState(book.title)
  const [author, setAuthor] = useState(book.author)
  const [description, setDescription] = useState(book.description ?? '')
  const [language, setLanguage] = useState(book.language)
  return (
    <Dialog
      open
      onClose={onClose}
      title={t('library.metadata')}
      closeLabel={t('common.close')}
      footer={
        <>
          <Button onClick={onClose}>{t('common.cancel')}</Button>
          <Button
            variant="accent"
            loading={loading}
            onClick={() => onSave({ title, author, description, language })}
          >
            {t('common.save')}
          </Button>
        </>
      }
    >
      <div style={{ display: 'grid', gap: 16 }}>
        <Field label={t('library.tableBook')}>
          <Input value={title} onChange={(event) => setTitle(event.target.value)} />
        </Field>
        <Field label={t('book.about')}>
          <Input value={author} onChange={(event) => setAuthor(event.target.value)} />
        </Field>
        <Field label={t('book.language')}>
          <Input value={language} onChange={(event) => setLanguage(event.target.value)} />
        </Field>
        <Field label={t('book.about')}>
          <Textarea value={description} onChange={(event) => setDescription(event.target.value)} />
        </Field>
      </div>
    </Dialog>
  )
}
