import { useQueries } from '@tanstack/react-query'
import { Edit3, ExternalLink, Highlighter, NotebookPen, Trash2 } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { booksApi } from '../api/bookflow'
import {
  queryKeys,
  useBooks,
  useCreateNote,
  useDeleteHighlight,
  useUpdateHighlight
} from '../api/hooks'
import {
  AlertDialog,
  Button,
  Dialog,
  EmptyState,
  ErrorState,
  Field,
  Select,
  Skeleton,
  Textarea,
  useToast
} from '../shared/ui'
import { formatDate } from '../shared/format'
import type { Highlight, HighlightColor } from '../types/api'
import { decodeHtmlEntities } from '../reader/selectionText'
import styles from './pages.module.css'

type DisplayHighlight = Highlight & { book_title: string }

export function HighlightsPage() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const { notify } = useToast()
  const [editing, setEditing] = useState<DisplayHighlight>()
  const [deleting, setDeleting] = useState<DisplayHighlight>()
  const booksQuery = useBooks({ sort: 'last_read', limit: 100 })
  const readyBooks =
    booksQuery.data?.items.filter((book) => book.processing_status === 'ready') ?? []
  const highlightQueries = useQueries({
    queries: readyBooks.map((book) => ({
      queryKey: queryKeys.highlights(book.id),
      queryFn: () => booksApi.highlights(book.id),
      staleTime: 60_000
    }))
  })
  const highlights: DisplayHighlight[] = highlightQueries
    .flatMap((query, index) =>
      (query.data?.items ?? []).map((highlight) => ({
        ...highlight,
        book_title: highlight.book_title ?? readyBooks[index]?.title ?? t('library.tableBook')
      }))
    )
    .sort((a, b) => b.created_at.localeCompare(a.created_at))
  const loading = booksQuery.isLoading || highlightQueries.some((query) => query.isLoading)
  const error = booksQuery.isError || highlightQueries.some((query) => query.isError)
  const createNote = useCreateNote()
  const removeHighlight = useDeleteHighlight()

  const addToNote = async (highlight: DisplayHighlight) => {
    const selectedText = decodeHtmlEntities(highlight.selected_text)
    const firstLine = selectedText.split('\n').find(Boolean) ?? selectedText
    const note = await createNote.mutateAsync({
      title: firstLine.slice(0, 72),
      book_id: highlight.book_id,
      highlight_id: highlight.id,
      blocks: [
        {
          id: crypto.randomUUID(),
          type: 'saved_quote',
          text: selectedText,
          book_id: highlight.book_id,
          locator: highlight.locator
        },
        { id: crypto.randomUUID(), type: 'paragraph', text: highlight.note ?? '' }
      ]
    })
    notify(t('highlights.addedToNote'), 'success')
    void navigate(`/notes?note=${note.id}`)
  }

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>{t('highlights.title')}</h1>
          <p className={styles.pageSubtitle}>{t('highlights.subtitle')}</p>
        </div>
        {highlights.length ? (
          <span className={styles.sectionMeta}>
            {t('highlights.count', { count: highlights.length })}
          </span>
        ) : null}
      </header>
      {loading ? (
        <div className={styles.highlightsList}>
          {Array.from({ length: 4 }, (_, index) => (
            <div key={index} className={styles.highlightCard}>
              <Skeleton width="35%" />
              <Skeleton height={72} />
              <Skeleton width="65%" />
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
        <div className={styles.highlightsList}>
          {highlights.map((highlight) => (
            <article
              key={highlight.id}
              className={styles.highlightCard}
              data-highlight-color={highlight.color}
            >
              <div className={styles.highlightHeader}>
                <div>
                  <p className={styles.highlightBook}>{highlight.book_title}</p>
                  <p className={styles.highlightDate}>
                    {formatDate(highlight.created_at, i18n.language)}
                  </p>
                </div>
                <div className={styles.highlightActions}>
                  <Button
                    size="small"
                    startIcon={NotebookPen}
                    loading={createNote.isPending}
                    onClick={() => void addToNote(highlight)}
                  >
                    {t('highlights.addToNote')}
                  </Button>
                  <Button size="small" startIcon={Edit3} onClick={() => setEditing(highlight)}>
                    {t('common.edit')}
                  </Button>
                  <Button
                    size="small"
                    variant="ghost"
                    startIcon={Trash2}
                    onClick={() => setDeleting(highlight)}
                  >
                    {t('common.delete')}
                  </Button>
                </div>
              </div>
              <blockquote className={styles.highlightQuote}>
                {decodeHtmlEntities(highlight.selected_text)}
              </blockquote>
              {highlight.note ? <p className={styles.highlightNote}>{highlight.note}</p> : null}
              <Link
                className={styles.highlightBookLink}
                to={`/read/${highlight.book_id}?chapter=${highlight.chapter_id ?? ''}&locator=${encodeURIComponent(highlight.locator)}`}
              >
                <ExternalLink size={14} aria-hidden="true" />
                {t('highlights.openInBook')}
              </Link>
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

      {editing ? (
        <EditHighlightDialog highlight={editing} onClose={() => setEditing(undefined)} />
      ) : null}
      <AlertDialog
        open={Boolean(deleting)}
        onClose={() => setDeleting(undefined)}
        onConfirm={() => {
          if (!deleting) return
          void removeHighlight
            .mutateAsync({ highlightId: deleting.id, bookId: deleting.book_id })
            .then(() => {
              notify(t('highlights.deleted'), 'success')
              setDeleting(undefined)
            })
        }}
        title={t('highlights.deleteTitle')}
        description={t('highlights.deleteConfirm')}
        confirmLabel={t('common.delete')}
        cancelLabel={t('common.cancel')}
        confirmLoading={removeHighlight.isPending}
      />
    </div>
  )
}

function EditHighlightDialog({
  highlight,
  onClose
}: {
  highlight: DisplayHighlight
  onClose: () => void
}) {
  const { t } = useTranslation()
  const { notify } = useToast()
  const update = useUpdateHighlight()
  const [text, setText] = useState(decodeHtmlEntities(highlight.selected_text))
  const [note, setNote] = useState(highlight.note ?? '')
  const [color, setColor] = useState<HighlightColor>(highlight.color)

  const save = async () => {
    if (!text.trim()) return
    await update.mutateAsync({
      highlightId: highlight.id,
      input: { selected_text: text.trim(), note: note.trim(), color }
    })
    notify(t('highlights.updated'), 'success')
    onClose()
  }

  return (
    <Dialog
      open
      onClose={onClose}
      title={t('highlights.editTitle')}
      description={highlight.book_title}
      closeLabel={t('common.close')}
      footer={
        <>
          <Button onClick={onClose}>{t('common.cancel')}</Button>
          <Button
            variant="accent"
            loading={update.isPending}
            disabled={!text.trim()}
            onClick={() => void save()}
          >
            {t('common.save')}
          </Button>
        </>
      }
    >
      <div className={styles.highlightEditForm}>
        <Field label={t('highlights.passage')}>
          <Textarea
            rows={7}
            aria-label={t('highlights.passage')}
            value={text}
            onChange={(event) => setText(event.target.value)}
          />
        </Field>
        <Field label={t('highlights.comment')}>
          <Textarea
            rows={3}
            aria-label={t('highlights.comment')}
            value={note}
            placeholder={t('highlights.commentPlaceholder')}
            onChange={(event) => setNote(event.target.value)}
          />
        </Field>
        <Field label={t('highlights.color')}>
          <Select
            aria-label={t('highlights.color')}
            value={color}
            onChange={(event) => setColor(event.target.value as HighlightColor)}
          >
            <option value="sand">{t('highlights.colorSand')}</option>
            <option value="sage">{t('highlights.colorSage')}</option>
            <option value="blue">{t('highlights.colorBlue')}</option>
            <option value="rose">{t('highlights.colorRose')}</option>
          </Select>
        </Field>
      </div>
    </Dialog>
  )
}
