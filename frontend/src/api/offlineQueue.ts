import { del, get, set } from 'idb-keyval'
import { booksApi } from './bookflow'
import { ApiError } from './http'
import type { ProgressUpdate } from '../types/api'

const KEY_PREFIX = 'bookflow:progress-queue:v1'
const OWNER_KEY = 'bookflow:offline-owner:v1'
const CHAPTER_CACHE = 'bookflow-last-chapters'

function queueKey(owner = localStorage.getItem(OWNER_KEY) ?? 'anonymous'): string {
  return `${KEY_PREFIX}:${owner}`
}

export interface QueuedProgress {
  id: string
  bookId: string
  update: ProgressUpdate
  queuedAt: string
}

export async function readProgressQueue(): Promise<QueuedProgress[]> {
  return (await get<QueuedProgress[]>(queueKey())) ?? []
}

export async function enqueueProgress(bookId: string, update: ProgressUpdate): Promise<void> {
  const queue = await readProgressQueue()
  const withoutOlderBookUpdates = queue.filter((item) => item.bookId !== bookId)
  await set(queueKey(), [
    ...withoutOlderBookUpdates,
    { id: crypto.randomUUID(), bookId, update, queuedAt: new Date().toISOString() }
  ])
}

export async function flushProgressQueue(): Promise<number> {
  if (!navigator.onLine) return 0
  const queue = await readProgressQueue()
  const pending: QueuedProgress[] = []
  let completed = 0

  for (const item of queue) {
    try {
      await booksApi.updateProgress(item.bookId, item.update)
      completed += 1
    } catch (error) {
      // A conflict means the server already has a newer position. Retrying the
      // stale item forever would create noise and could never succeed.
      if (error instanceof ApiError && error.status === 409) completed += 1
      else pending.push(item)
    }
  }

  if (pending.length) await set(queueKey(), pending)
  else await del(queueKey())
  return completed
}

export async function clearProgressQueue(owner?: string): Promise<void> {
  await del(queueKey(owner))
}

async function clearChapterCaches(): Promise<void> {
  if (!('caches' in globalThis)) return
  const names = await caches.keys()
  await Promise.all(
    names
      .filter((name) => name.includes(CHAPTER_CACHE))
      .map(async (name) => await caches.delete(name))
  )
}

export async function ensureOfflineOwner(userId: string): Promise<void> {
  const previous = localStorage.getItem(OWNER_KEY)
  if (previous && previous !== userId) {
    await clearProgressQueue(previous)
    await clearChapterCaches()
  }
  localStorage.setItem(OWNER_KEY, userId)
}

export async function clearPrivateOfflineData(): Promise<void> {
  const owner = localStorage.getItem(OWNER_KEY) ?? undefined
  await clearProgressQueue(owner)
  await clearChapterCaches()
  localStorage.removeItem(OWNER_KEY)
}
