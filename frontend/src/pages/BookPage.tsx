import { ImagePlus, BookOpenText, Clock3, MessageSquareText, Star, Trash2 } from 'lucide-react'
import { useRef, type ChangeEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { BookCover } from '../entities/BookCard'
import {
  useBook,
  useBookBookmarks,
  useBookHighlights,
  useBookToc,
  useCurrentUser,
  useDictionary,
  useMutateBook,
  useNotes,
  useRemoveBookCover,
  useUploadBookCover,
  useStatistics
} from '../api/hooks'
import {
  Badge,
  Breadcrumbs,
  Button,
  EmptyState,
  ErrorState,
  LoadingState,
  ProgressBar,
  Tabs
} from '../shared/ui'
import { formatDate, formatDateTime, formatDuration } from '../shared/format'
import styles from './pages.module.css'
import { decodeHtmlEntities } from '../reader/selectionText'

export function BookPage() {
  const { bookId } = useParams()
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const user = useCurrentUser().data?.user
  const bookQuery = useBook(bookId)
  const tocQuery = useBookToc(bookId)
  const bookmarksQuery = useBookBookmarks(bookId)
  const highlightsQuery = useBookHighlights(bookId)
  const notesQuery = useNotes({ book_id: bookId })
  const dictionaryQuery = useDictionary({ book_id: bookId, limit: 10 })
  const statistics = useStatistics(
    user?.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone,
    30
  )
  const mutateBook = useMutateBook()
  const uploadCover = useUploadBookCover(bookId ?? '')
  const removeCover = useRemoveBookCover(bookId ?? '')
  const coverInput = useRef<HTMLInputElement>(null)

  const handleCoverChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (file) void uploadCover.mutateAsync(file)
  }

  if (bookQuery.isLoading) return <LoadingState label={t('common.loading')} />
  if (bookQuery.isError || !bookQuery.data) {
    return (
      <ErrorState
        title={t('common.errorTitle')}
        body={t('common.errorMessage')}
        retryLabel={t('common.retry')}
        onRetry={() => void bookQuery.refetch()}
      />
    )
  }

  const book = bookQuery.data
  const ready = book.processing_status === 'ready'
  const firstChapter = book.current_chapter_id ?? tocQuery.data?.[0]?.chapter_id
  const readPath = firstChapter ? `/read/${book.id}?chapter=${firstChapter}` : `/read/${book.id}`
  const sessions =
    statistics.sessions.data?.items.filter((session) => session.book_id === book.id).slice(0, 5) ??
    []
  const bookStat = statistics.books.data?.find((item) => item.book_id === book.id)

  const overview = (
    <div>
      {book.description ? <p className={styles.bookDescription}>{book.description}</p> : null}
      <section className={styles.section} aria-labelledby="book-statistics">
        <div className={styles.sectionHeader}>
          <h2 id="book-statistics" className={styles.sectionTitle}>
            {t('nav.statistics')}
          </h2>
        </div>
        <div className={styles.statsGrid}>
          <Metric
            label={t('statistics.activeTime')}
            value={formatDuration(bookStat?.active_seconds ?? 0, i18n.language)}
          />
          <Metric
            label={t('statistics.sessions')}
            value={String(bookStat?.sessions_count ?? sessions.length)}
          />
          <Metric
            label={t('statistics.averageSpeed')}
            value={t('statistics.wpm', {
              count: Math.round(bookStat?.average_words_per_minute ?? 0)
            })}
          />
          <Metric
            label={t('dictionary.title')}
            value={String(
              dictionaryQuery.data?.total_count ?? dictionaryQuery.data?.items.length ?? 0
            )}
          />
        </div>
      </section>
      <section className={styles.section} aria-labelledby="recent-sessions">
        <div className={styles.sectionHeader}>
          <h2 id="recent-sessions" className={styles.sectionTitle}>
            {t('book.sessions')}
          </h2>
        </div>
        {sessions.length ? (
          <div className={styles.flatList}>
            {sessions.map((session) => (
              <div key={session.id} className={styles.flatRow}>
                <div>
                  <p className={styles.flatTitle}>
                    {formatDateTime(session.started_at, i18n.language)}
                  </p>
                  <p className={styles.flatMeta}>{session.close_reason ?? session.status}</p>
                </div>
                <strong>{formatDuration(session.active_seconds, i18n.language)}</strong>
              </div>
            ))}
          </div>
        ) : (
          <EmptyState
            icon={Clock3}
            title={t('statistics.noActivity')}
            body={t('statistics.subtitle')}
          />
        )}
      </section>
    </div>
  )

  const contents = tocQuery.isLoading ? (
    <LoadingState label={t('common.loading')} />
  ) : tocQuery.data?.length ? (
    <div className={styles.tocList}>
      {tocQuery.data.map((item, index) => (
        <Link
          key={item.id}
          className={styles.tocLink}
          style={{ paddingLeft: `${8 + item.level * 12}px` }}
          to={`/read/${book.id}?chapter=${item.chapter_id}`}
        >
          <span>{item.title}</span>
          <span className={styles.tocOrder}>{index + 1}</span>
        </Link>
      ))}
    </div>
  ) : (
    <EmptyState
      icon={BookOpenText}
      title={t('book.processingTitle')}
      body={t('book.processingBody')}
    />
  )

  const bookmarks = bookmarksQuery.data?.items.length ? (
    <div className={styles.flatList}>
      {bookmarksQuery.data.items.map((bookmark) => (
        <Link
          key={bookmark.id}
          className={styles.flatRow}
          to={`/read/${book.id}?chapter=${bookmark.chapter_id}&locator=${encodeURIComponent(bookmark.locator)}`}
        >
          <div>
            <p className={styles.flatTitle}>{bookmark.title}</p>
            <p className={styles.flatMeta}>
              {bookmark.note ??
                t('library.progress', { value: Math.round(bookmark.progress_percent) })}
            </p>
          </div>
          <span>{formatDate(bookmark.created_at, i18n.language)}</span>
        </Link>
      ))}
    </div>
  ) : (
    <EmptyState title={t('book.noBookmarks')} body={t('reader.bookmark')} />
  )

  const highlights = highlightsQuery.data?.items.length ? (
    <div className={styles.flatList}>
      {highlightsQuery.data.items.map((highlight) => (
        <Link
          key={highlight.id}
          className={styles.flatRow}
          to={`/read/${book.id}?chapter=${highlight.chapter_id}&locator=${encodeURIComponent(highlight.locator)}`}
        >
          <blockquote className={styles.quote}>
            {decodeHtmlEntities(highlight.selected_text)}
          </blockquote>
          <span>{formatDate(highlight.created_at, i18n.language)}</span>
        </Link>
      ))}
    </div>
  ) : (
    <EmptyState title={t('book.noHighlights')} body={t('highlights.emptyBody')} />
  )

  const notes = notesQuery.data?.items.length ? (
    <div className={styles.flatList}>
      {notesQuery.data.items.map((note) => (
        <Link key={note.id} className={styles.flatRow} to={`/notes?note=${note.id}`}>
          <div>
            <p className={styles.flatTitle}>{note.title}</p>
            <p className={styles.flatMeta}>{note.blocks[0]?.text ?? ''}</p>
          </div>
          <span>{formatDate(note.updated_at, i18n.language)}</span>
        </Link>
      ))}
    </div>
  ) : (
    <EmptyState icon={MessageSquareText} title={t('book.noNotes')} body={t('notes.emptyBody')} />
  )

  return (
    <div className={styles.page}>
      <Breadcrumbs items={[{ label: t('nav.library'), href: '/library' }, { label: book.title }]} />
      <div className={styles.bookHero}>
        <aside className={styles.bookCoverColumn}>
          <BookCover book={book} className={styles.bookDetailCover} />
          <input
            ref={coverInput}
            className={styles.visuallyHiddenInput}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            aria-label={t('book.coverUpload')}
            onChange={handleCoverChange}
          />
          <div className={styles.bookCoverActions}>
            <Button
              startIcon={ImagePlus}
              loading={uploadCover.isPending}
              disabled={removeCover.isPending}
              onClick={() => coverInput.current?.click()}
            >
              {book.has_custom_cover ? t('book.coverReplace') : t('book.coverUpload')}
            </Button>
            {book.has_custom_cover ? (
              <Button
                variant="danger"
                startIcon={Trash2}
                loading={removeCover.isPending}
                disabled={uploadCover.isPending}
                onClick={() => void removeCover.mutateAsync()}
              >
                {t('book.coverRemove')}
              </Button>
            ) : null}
          </div>
          {uploadCover.isError || removeCover.isError ? (
            <p className={styles.coverUploadError} role="alert">
              {t('book.coverUploadError')}
            </p>
          ) : null}
          <p className={styles.coverUploadHint}>{t('book.coverUploadHint')}</p>
        </aside>
        <article className={styles.bookDocument}>
          <h1 className={styles.bookTitle}>{book.title}</h1>
          <p className={styles.bookAuthor}>{book.author}</p>
          <div className={styles.bookActions}>
            <Button
              variant="accent"
              startIcon={BookOpenText}
              disabled={!ready || !firstChapter}
              onClick={() => navigate(readPath)}
            >
              {book.progress_percent > 0 ? t('book.read') : t('book.start')}
            </Button>
            <Button
              startIcon={Star}
              onClick={() =>
                void mutateBook.mutateAsync({
                  bookId: book.id,
                  input: { is_favorite: !book.is_favorite }
                })
              }
            >
              {book.is_favorite ? t('library.unfavorite') : t('library.favorite')}
            </Button>
            <Badge
              tone={book.processing_status === 'failed' ? 'danger' : ready ? 'accent' : 'neutral'}
            >
              {t(`library.${book.processing_status}`)}
            </Badge>
          </div>
          {book.processing_status === 'failed' ? (
            <div className={styles.formError} role="alert">
              {book.processing_error ?? t('book.failedTitle')}
            </div>
          ) : null}
          <dl className={styles.bookFacts}>
            <Fact label={t('book.format')} value={book.format.toUpperCase()} />
            <Fact label={t('book.language')} value={book.language.toUpperCase()} />
            <Fact
              label={t('book.added', { date: '' }).replace(/\s+$/, '')}
              value={formatDate(book.added_at, i18n.language)}
            />
            <div className={styles.fact}>
              <dt>{t('book.progress')}</dt>
              <dd>
                <ProgressBar value={book.progress_percent} label={t('book.progress')} />
                <span>{Math.round(book.progress_percent)}%</span>
              </dd>
            </div>
          </dl>
          {!ready ? (
            <EmptyState
              icon={BookOpenText}
              title={
                book.processing_status === 'failed'
                  ? t('book.failedTitle')
                  : t('book.processingTitle')
              }
              body={book.processing_error ?? t('book.processingBody')}
            />
          ) : (
            <Tabs
              items={[
                { id: 'overview', label: t('book.overview'), content: overview },
                { id: 'contents', label: t('book.contents'), content: contents },
                { id: 'bookmarks', label: t('book.bookmarks'), content: bookmarks },
                { id: 'highlights', label: t('book.highlights'), content: highlights },
                { id: 'notes', label: t('book.notes'), content: notes }
              ]}
            />
          )}
        </article>
      </div>
    </div>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className={styles.fact}>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  )
}
function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className={styles.metric}>
      <span className={styles.metricLabel}>{label}</span>
      <strong className={styles.metricValue}>{value}</strong>
    </div>
  )
}
