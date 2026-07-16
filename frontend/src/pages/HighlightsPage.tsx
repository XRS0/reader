import { useQueries } from '@tanstack/react-query'
import { Highlighter } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { booksApi } from '../api/bookflow'
import { queryKeys, useBooks } from '../api/hooks'
import { EmptyState, ErrorState, Skeleton } from '../shared/ui'
import { formatDate } from '../shared/format'
import styles from './pages.module.css'

export function HighlightsPage() {
  const { t, i18n } = useTranslation()
  const booksQuery = useBooks({ sort: 'last_read', limit: 50 })
  const readyBooks =
    booksQuery.data?.items.filter((book) => book.processing_status === 'ready') ?? []
  const highlightQueries = useQueries({
    queries: readyBooks.map((book) => ({
      queryKey: queryKeys.highlights(book.id),
      queryFn: () => booksApi.highlights(book.id),
      staleTime: 60_000
    }))
  })
  const highlights = highlightQueries
    .flatMap((query) => query.data?.items ?? [])
    .sort((a, b) => b.created_at.localeCompare(a.created_at))
  const loading = booksQuery.isLoading || highlightQueries.some((query) => query.isLoading)
  const error = booksQuery.isError || highlightQueries.some((query) => query.isError)

  return (
    <div className={`${styles.page} ${styles.pageNarrow}`}>
      <header className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>{t('highlights.title')}</h1>
          <p className={styles.pageSubtitle}>{t('highlights.subtitle')}</p>
        </div>
      </header>
      {loading ? (
        <div className={styles.flatList}>
          {Array.from({ length: 6 }, (_, index) => (
            <div key={index} className={styles.flatRow}>
              <Skeleton height={54} />
              <Skeleton width={80} />
            </div>
          ))}
        </div>
      ) : error ? (
        <ErrorState
          title={t('common.errorTitle')}
          body={t('common.errorMessage')}
          retryLabel={t('common.retry')}
          onRetry={() => void booksQuery.refetch()}
        />
      ) : highlights.length ? (
        <div className={styles.flatList}>
          {highlights.map((highlight) => (
            <article key={highlight.id} className={styles.flatRow}>
              <div>
                <blockquote className={styles.quote}>{highlight.selected_text}</blockquote>
                <p className={styles.flatMeta}>
                  {highlight.book_title}
                  {highlight.note ? ` · ${highlight.note}` : ''}
                </p>
              </div>
              <div style={{ textAlign: 'right' }}>
                <span className={styles.flatMeta}>
                  {formatDate(highlight.created_at, i18n.language)}
                </span>
                <Link
                  className={styles.flatTitle}
                  to={`/read/${highlight.book_id}?chapter=${highlight.chapter_id}&locator=${encodeURIComponent(highlight.locator)}`}
                >
                  {t('highlights.openInBook')}
                </Link>
              </div>
            </article>
          ))}
        </div>
      ) : (
        <EmptyState
          icon={Highlighter}
          title={t('highlights.emptyTitle')}
          body={t('highlights.emptyBody')}
        />
      )}
    </div>
  )
}
