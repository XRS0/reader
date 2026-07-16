import { Download, Edit3, MoreHorizontal, Play, RefreshCw, Star, Trash2 } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { booksApi } from '../api/bookflow'
import {
  Badge,
  Button,
  ContextMenu,
  DropdownMenu,
  IconButton,
  ProgressBar,
  type MenuItem
} from '../shared/ui'
import type { Book } from '../types/api'
import styles from './book.module.css'

export function BookCover({ book, className }: { book: Book; className?: string }) {
  const tone =
    (book.id.split('').reduce((sum, character) => sum + character.charCodeAt(0), 0) % 4) + 1
  return (
    <div className={clsx(styles.cover, className)} data-tone={tone} aria-hidden="true">
      {book.cover_url ? (
        <img className={styles.coverImage} src={book.cover_url} alt="" loading="lazy" />
      ) : (
        <span className={styles.coverLetter}>{book.title.trim()[0]?.toUpperCase() ?? 'B'}</span>
      )}
      <span className={styles.coverFormat}>{book.format}</span>
    </div>
  )
}

function statusLabel(book: Book, t: (key: string) => string) {
  return t(`library.${book.processing_status}`)
}

export interface BookCardProps {
  book: Book
  view?: 'grid' | 'list'
  onFavorite?: (book: Book) => void
  onDelete?: (book: Book) => void
  onEdit?: (book: Book) => void
  onReprocess?: (book: Book) => void
}

export function BookCard({
  book,
  view = 'grid',
  onFavorite,
  onDelete,
  onEdit,
  onReprocess
}: BookCardProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const ready = book.processing_status === 'ready'
  const openPath = ready ? `/read/${book.id}` : `/books/${book.id}`
  const items: MenuItem[] = [
    {
      id: 'open',
      label: t('common.open'),
      icon: Play,
      onSelect: () => void navigate(`/books/${book.id}`)
    },
    {
      id: 'continue',
      label: t('library.continueReading'),
      icon: Play,
      disabled: !ready,
      onSelect: () => void navigate(`/read/${book.id}`)
    },
    {
      id: 'favorite',
      label: book.is_favorite ? t('library.unfavorite') : t('library.favorite'),
      icon: Star,
      onSelect: () => onFavorite?.(book)
    },
    { id: 'edit', label: t('library.metadata'), icon: Edit3, onSelect: () => onEdit?.(book) },
    {
      id: 'download',
      label: t('library.download'),
      icon: Download,
      onSelect: () =>
        void booksApi.download(book.id).then((url) => {
          window.location.assign(url)
        })
    },
    {
      id: 'reprocess',
      label: t('library.reprocess'),
      icon: RefreshCw,
      separatorBefore: true,
      onSelect: () => onReprocess?.(book)
    },
    {
      id: 'delete',
      label: t('common.delete'),
      icon: Trash2,
      danger: true,
      onSelect: () => onDelete?.(book)
    }
  ]
  const menu = (
    <DropdownMenu
      label={t('common.more')}
      items={items}
      trigger={
        <IconButton
          className={styles.more}
          size="small"
          icon={MoreHorizontal}
          label={t('common.more')}
        />
      }
    />
  )

  if (view === 'list') {
    return (
      <ContextMenu items={items} label={t('common.more')}>
        <article className={styles.listCard}>
          <Link to={openPath}>
            <BookCover book={book} />
          </Link>
          <div className={styles.cardInfo}>
            <div className={styles.cardTitleRow}>
              <Link to={openPath} className={styles.title}>
                {book.title}
              </Link>
              {book.is_favorite ? (
                <Star size={13} fill="currentColor" aria-label={t('nav.favorites')} />
              ) : null}
            </div>
            <p className={styles.author}>{book.author}</p>
            <div className={styles.progress}>
              <ProgressBar
                value={book.progress_percent}
                label={t('library.progress', { value: Math.round(book.progress_percent) })}
              />
            </div>
            <div className={styles.meta}>
              <span>{t('library.progress', { value: Math.round(book.progress_percent) })}</span>
              <span>
                {book.estimated_minutes_remaining
                  ? t('library.remaining', { count: book.estimated_minutes_remaining })
                  : statusLabel(book, t)}
              </span>
            </div>
          </div>
          <div className={styles.listActions}>
            {ready ? (
              <Button variant="ghost" size="small" onClick={() => navigate(`/read/${book.id}`)}>
                {t('book.read')}
              </Button>
            ) : (
              <Badge tone={book.processing_status === 'failed' ? 'danger' : 'neutral'}>
                {statusLabel(book, t)}
              </Badge>
            )}
            {menu}
          </div>
        </article>
      </ContextMenu>
    )
  }

  return (
    <ContextMenu items={items} label={t('common.more')}>
      <article className={styles.card}>
        {book.is_favorite ? (
          <span className={styles.favorite}>
            <Star size={13} fill="currentColor" />
          </span>
        ) : null}
        <Link to={openPath} className={styles.cardLink}>
          <BookCover book={book} className={styles.cardCover} />
        </Link>
        <div className={styles.cardInfo}>
          <div className={styles.cardTitleRow}>
            <Link to={openPath} className={styles.title}>
              {book.title}
            </Link>
            {menu}
          </div>
          <p className={styles.author}>{book.author}</p>
          {ready ? (
            <>
              <ProgressBar
                value={book.progress_percent}
                label={t('library.progress', { value: Math.round(book.progress_percent) })}
              />
              <div className={styles.meta}>
                <span>{Math.round(book.progress_percent)}%</span>
                <span>
                  {book.estimated_minutes_remaining
                    ? t('common.minutes', { count: book.estimated_minutes_remaining })
                    : ''}
                </span>
              </div>
            </>
          ) : (
            <Badge tone={book.processing_status === 'failed' ? 'danger' : 'neutral'}>
              {statusLabel(book, t)}
            </Badge>
          )}
        </div>
      </article>
    </ContextMenu>
  )
}
