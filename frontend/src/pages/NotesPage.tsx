import { FilePlus2, NotebookPen, Plus } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { useCreateNote, useDebouncedValue, useNotes, useUpdateNote } from '../api/hooks'
import {
  Button,
  EmptyState,
  ErrorState,
  SearchInput,
  Select,
  Skeleton,
  useToast
} from '../shared/ui'
import { formatDate } from '../shared/format'
import type { Note, NoteBlock, NoteBlockType } from '../types/api'
import styles from './pages.module.css'

export function NotesPage() {
  const { t, i18n } = useTranslation()
  const [params, setParams] = useSearchParams()
  const [search, setSearch] = useState('')
  const debounced = useDebouncedValue(search)
  const query = useNotes({ search: debounced })
  const selectedId = params.get('note') ?? undefined
  const [draft, setDraft] = useState<Note>()

  useEffect(() => {
    if (params.get('new') === '1' && !draft) {
      setDraft({
        id: 'draft',
        title: '',
        schema_version: 1,
        blocks: [{ id: crypto.randomUUID(), type: 'paragraph', text: '' }],
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString()
      })
    }
  }, [draft, params])

  useEffect(() => {
    if (selectedId && query.data) setDraft(query.data.items.find((note) => note.id === selectedId))
  }, [query.data, selectedId])

  const newNote = () => {
    setDraft({
      id: 'draft',
      title: '',
      schema_version: 1,
      blocks: [{ id: crypto.randomUUID(), type: 'paragraph', text: '' }],
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString()
    })
    setParams({ new: '1' })
  }

  return (
    <div className={clsx(styles.page, styles.pageNarrow)}>
      <header className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>{t('notes.title')}</h1>
          <p className={styles.pageSubtitle}>{t('notes.emptyBody')}</p>
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
        <div className={styles.notesLayout}>
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
            {query.isLoading
              ? Array.from({ length: 5 }, (_, index) => (
                  <div key={index} className={styles.noteItem}>
                    <Skeleton width="80%" />
                    <div style={{ height: 6 }} />
                    <Skeleton width="45%" height={10} />
                  </div>
                ))
              : query.data?.items.map((note) => (
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
                    <span className={styles.noteItemMeta}>
                      {formatDate(note.updated_at, i18n.language)}
                      {note.book_title ? ` · ${note.book_title}` : ''}
                    </span>
                  </button>
                ))}
          </aside>
          <section className={styles.noteEditor} aria-label={t('notes.title')}>
            {draft ? (
              <NoteEditor
                key={draft.id}
                note={draft}
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
    </div>
  )
}

function NoteEditor({ note, onSaved }: { note: Note; onSaved: (note: Note) => void }) {
  const { t } = useTranslation()
  const { notify } = useToast()
  const [title, setTitle] = useState(note.title)
  const [blocks, setBlocks] = useState(note.blocks)
  const [newType, setNewType] = useState<NoteBlockType>('paragraph')
  const create = useCreateNote()
  const update = useUpdateNote(note.id === 'draft' ? undefined : note.id)
  const saving = create.isPending || update.isPending
  const updateBlock = (id: string, input: Partial<NoteBlock>) =>
    setBlocks((items) => items.map((block) => (block.id === id ? { ...block, ...input } : block)))
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
  return (
    <div>
      <input
        className={styles.noteTitleInput}
        aria-label={t('notes.untitled')}
        placeholder={t('notes.untitled')}
        value={title}
        onChange={(event) => setTitle(event.target.value)}
      />
      <div className={styles.blockEditor}>
        {blocks.map((block) =>
          block.type === 'divider' ? (
            <hr key={block.id} />
          ) : (
            <div key={block.id} style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
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
                rows={block.type.startsWith('heading') ? 1 : 2}
                style={{
                  fontSize:
                    block.type === 'heading1' ? 24 : block.type === 'heading2' ? 20 : undefined,
                  fontStyle:
                    block.type === 'quote' || block.type === 'saved_quote' ? 'italic' : undefined
                }}
                placeholder={t('notes.editorHint')}
                value={block.text ?? ''}
                onChange={(event) => updateBlock(block.id, { text: event.target.value })}
              />
            </div>
          )
        )}
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
        <Button variant="accent" loading={saving} onClick={() => void save()}>
          {t('common.save')}
        </Button>
      </div>
    </div>
  )
}
