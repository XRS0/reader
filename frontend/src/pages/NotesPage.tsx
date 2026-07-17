import { ArrowLeft, FilePlus2, GripVertical, NotebookPen, Plus, Save, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import {
  useBooks,
  useCreateNote,
  useDebouncedValue,
  useDeleteNote,
  useNotes,
  useUpdateNote
} from '../api/hooks'
import {
  AlertDialog,
  Button,
  EmptyState,
  ErrorState,
  IconButton,
  SearchInput,
  Select,
  Skeleton,
  useToast
} from '../shared/ui'
import { formatDate } from '../shared/format'
import type { Note, NoteBlock, NoteBlockType } from '../types/api'
import { decodeHtmlEntities } from '../reader/selectionText'
import styles from './pages.module.css'

function createDraft(): Note {
  return {
    id: 'draft',
    title: '',
    schema_version: 1,
    blocks: [{ id: crypto.randomUUID(), type: 'paragraph', text: '' }],
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString()
  }
}

function notePreview(note: Note): string {
  return decodeHtmlEntities(
    note.blocks
      .find((block) => block.text?.trim())
      ?.text?.replace(/\s+/g, ' ')
      .trim() ?? ''
  )
}

export function NotesPage() {
  const { t, i18n } = useTranslation()
  const [params, setParams] = useSearchParams()
  const [search, setSearch] = useState('')
  const debounced = useDebouncedValue(search)
  const query = useNotes({ search: debounced })
  const booksQuery = useBooks({ limit: 100, sort: 'title' })
  const selectedId = params.get('note') ?? undefined
  const [draft, setDraft] = useState<Note>()
  const [deleting, setDeleting] = useState<Note>()
  const remove = useDeleteNote()
  const { notify } = useToast()
  const bookTitles = useMemo(
    () => new Map((booksQuery.data?.items ?? []).map((book) => [book.id, book.title])),
    [booksQuery.data?.items]
  )

  useEffect(() => {
    if (params.get('new') === '1' && !draft) setDraft(createDraft())
  }, [draft, params])

  useEffect(() => {
    if (selectedId && query.data) setDraft(query.data.items.find((note) => note.id === selectedId))
  }, [query.data, selectedId])

  const newNote = () => {
    setDraft(createDraft())
    setParams({ new: '1' })
  }

  const closeEditor = () => {
    setDraft(undefined)
    setParams({})
  }

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>{t('notes.title')}</h1>
          <p className={styles.pageSubtitle}>
            {query.data
              ? t('notes.count', { count: query.data.items.length })
              : t('notes.emptyBody')}
          </p>
        </div>
        <Button variant="accent" startIcon={Plus} onClick={newNote}>
          {t('notes.newNote')}
        </Button>
      </header>
      {query.isError ? (
        <ErrorState
          title={t('common.errorTitle')}
          body={t('common.errorMessage')}
          retryLabel={t('common.retry')}
          onRetry={() => void query.refetch()}
        />
      ) : (
        <div className={clsx(styles.notesLayout, draft && styles.notesLayoutSelected)}>
          <aside className={styles.notesList} aria-label={t('notes.title')}>
            <div className={styles.notesListHeader}>
              <SearchInput
                label={t('common.search')}
                placeholder={t('notes.searchPlaceholder')}
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                onClear={() => setSearch('')}
              />
            </div>
            <div className={styles.notesItems}>
              {query.isLoading
                ? Array.from({ length: 5 }, (_, index) => (
                    <div key={index} className={styles.noteItem}>
                      <Skeleton width="80%" />
                      <div style={{ height: 8 }} />
                      <Skeleton width="95%" height={10} />
                      <div style={{ height: 6 }} />
                      <Skeleton width="45%" height={10} />
                    </div>
                  ))
                : query.data?.items.map((note) => {
                    const preview = notePreview(note)
                    const bookTitle =
                      note.book_title ?? (note.book_id ? bookTitles.get(note.book_id) : undefined)
                    return (
                      <button
                        key={note.id}
                        type="button"
                        className={clsx(
                          styles.noteItem,
                          draft?.id === note.id && styles.noteItemActive
                        )}
                        onClick={() => {
                          setDraft(note)
                          setParams({ note: note.id })
                        }}
                      >
                        <span className={styles.noteItemTitle}>
                          {note.title || t('notes.untitled')}
                        </span>
                        {preview ? <span className={styles.noteItemPreview}>{preview}</span> : null}
                        <span className={styles.noteItemMeta}>
                          {formatDate(note.updated_at, i18n.language)}
                          {bookTitle ? ` · ${bookTitle}` : ''}
                        </span>
                      </button>
                    )
                  })}
            </div>
          </aside>
          <section className={styles.noteEditor} aria-label={t('notes.editor')}>
            {draft ? (
              <NoteEditor
                key={draft.id}
                note={draft}
                bookTitle={
                  draft.book_title ?? (draft.book_id ? bookTitles.get(draft.book_id) : undefined)
                }
                onBack={closeEditor}
                onDelete={draft.id === 'draft' ? undefined : () => setDeleting(draft)}
                onSaved={(note) => {
                  setDraft(note)
                  setParams({ note: note.id })
                }}
              />
            ) : (
              <EmptyState
                icon={NotebookPen}
                title={query.data?.items.length ? t('notes.selectTitle') : t('notes.emptyTitle')}
                body={query.data?.items.length ? t('notes.selectBody') : t('notes.emptyBody')}
                action={
                  <Button startIcon={FilePlus2} onClick={newNote}>
                    {t('notes.newNote')}
                  </Button>
                }
              />
            )}
          </section>
        </div>
      )}
      <AlertDialog
        open={Boolean(deleting)}
        onClose={() => setDeleting(undefined)}
        onConfirm={() => {
          if (!deleting) return
          void remove.mutateAsync(deleting.id).then(() => {
            notify(t('notes.deleted'), 'success')
            setDeleting(undefined)
            closeEditor()
          })
        }}
        title={t('notes.deleteTitle')}
        description={t('notes.deleteConfirm')}
        confirmLabel={t('common.delete')}
        cancelLabel={t('common.cancel')}
        confirmLoading={remove.isPending}
      />
    </div>
  )
}

function NoteEditor({
  note,
  bookTitle,
  onSaved,
  onDelete,
  onBack
}: {
  note: Note
  bookTitle?: string
  onSaved: (note: Note) => void
  onDelete?: () => void
  onBack: () => void
}) {
  const { t, i18n } = useTranslation()
  const { notify } = useToast()
  const [title, setTitle] = useState(note.title)
  const [blocks, setBlocks] = useState(
    (note.blocks.length ? note.blocks : createDraft().blocks).map((block) => ({
      ...block,
      text: block.text ? decodeHtmlEntities(block.text) : block.text
    }))
  )
  const [newType, setNewType] = useState<NoteBlockType>('paragraph')
  const create = useCreateNote()
  const update = useUpdateNote(note.id === 'draft' ? undefined : note.id)
  const saving = create.isPending || update.isPending
  const updateBlock = (id: string, input: Partial<NoteBlock>) =>
    setBlocks((items) => items.map((block) => (block.id === id ? { ...block, ...input } : block)))
  const removeBlock = (id: string) =>
    setBlocks((items) => {
      const next = items.filter((block) => block.id !== id)
      return next.length ? next : [{ id: crypto.randomUUID(), type: 'paragraph', text: '' }]
    })
  const save = async () => {
    const value =
      note.id === 'draft'
        ? await create.mutateAsync({ title: title || t('notes.untitled'), blocks })
        : await update.mutateAsync({ title: title || t('notes.untitled'), blocks })
    notify(t('settings.saved'), 'success')
    onSaved(value)
  }
  const options = useMemo<Array<{ value: NoteBlockType; label: string }>>(
    () => [
      { value: 'paragraph', label: t('notes.paragraph') },
      { value: 'heading1', label: t('notes.heading') },
      { value: 'bulleted_list', label: t('notes.bulletedList') },
      { value: 'numbered_list', label: t('notes.numberedList') },
      { value: 'task', label: t('notes.task') },
      { value: 'quote', label: t('notes.quote') },
      { value: 'callout', label: t('notes.callout') },
      { value: 'divider', label: t('notes.divider') },
      { value: 'link', label: t('notes.link') },
      { value: 'book_link', label: t('notes.bookLink') },
      { value: 'saved_quote', label: t('notes.savedQuote') }
    ],
    [t]
  )
  const typeLabel = (type: NoteBlockType) =>
    options.find((option) => option.value === type)?.label ?? type

  return (
    <div className={styles.noteDocument}>
      <div className={styles.noteEditorHeader}>
        <Button variant="ghost" size="small" startIcon={ArrowLeft} onClick={onBack}>
          {t('common.back')}
        </Button>
        <div className={styles.noteEditorMeta}>
          <span>{bookTitle ?? t('notes.personal')}</span>
          <span>{t('notes.updated', { date: formatDate(note.updated_at, i18n.language) })}</span>
        </div>
        {onDelete ? (
          <IconButton icon={Trash2} label={t('common.delete')} onClick={onDelete} />
        ) : null}
      </div>
      <input
        className={styles.noteTitleInput}
        aria-label={t('notes.untitled')}
        placeholder={t('notes.untitled')}
        value={title}
        onChange={(event) => setTitle(event.target.value)}
      />
      <div className={styles.blockEditor}>
        {blocks.map((block) => (
          <div key={block.id} className={styles.noteBlock} data-block-type={block.type}>
            <div className={styles.noteBlockRail}>
              <GripVertical size={15} aria-hidden="true" />
              <span>{typeLabel(block.type)}</span>
            </div>
            {block.type === 'divider' ? (
              <hr className={styles.noteDivider} />
            ) : (
              <div className={styles.noteBlockContent}>
                {block.type === 'task' ? (
                  <input
                    type="checkbox"
                    checked={block.checked ?? false}
                    aria-label={t('notes.task')}
                    onChange={(event) => updateBlock(block.id, { checked: event.target.checked })}
                  />
                ) : null}
                <textarea
                  className={styles.blockInput}
                  aria-label={typeLabel(block.type)}
                  rows={block.type.startsWith('heading') ? 1 : block.type === 'saved_quote' ? 4 : 2}
                  placeholder={t('notes.editorHint')}
                  value={block.text ?? ''}
                  onChange={(event) => updateBlock(block.id, { text: event.target.value })}
                />
              </div>
            )}
            <IconButton
              className={styles.noteBlockDelete}
              size="small"
              icon={Trash2}
              label={t('notes.deleteBlock')}
              onClick={() => removeBlock(block.id)}
            />
          </div>
        ))}
      </div>
      <div className={styles.noteToolbar}>
        <Select
          className={styles.compactSelect}
          value={newType}
          aria-label={t('notes.addBlock')}
          onChange={(event) => setNewType(event.target.value as NoteBlockType)}
        >
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </Select>
        <Button
          startIcon={Plus}
          onClick={() =>
            setBlocks((items) => [
              ...items,
              {
                id: crypto.randomUUID(),
                type: newType,
                text: newType === 'divider' ? undefined : ''
              }
            ])
          }
        >
          {t('notes.addBlock')}
        </Button>
        <Button variant="accent" startIcon={Save} loading={saving} onClick={() => void save()}>
          {t('common.save')}
        </Button>
      </div>
    </div>
  )
}
