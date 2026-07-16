import { booksApi } from './bookflow'
import { ApiError } from './http'
import {
  clearProgressQueue,
  enqueueProgress,
  flushProgressQueue,
  readProgressQueue
} from './offlineQueue'

vi.mock('./bookflow', () => ({ booksApi: { updateProgress: vi.fn() } }))

const update = {
  chapter_id: 'chapter-1',
  locator_type: 'chapter_offset' as const,
  locator: 'chapter:1:offset:0',
  character_offset: 0,
  progress_percent: 10,
  scroll_percent: 10,
  revision: 1,
  client_id: 'client-1',
  client_timestamp: '2026-07-15T00:00:00Z'
}

describe('offline progress queue', () => {
  beforeEach(async () => {
    localStorage.setItem('bookflow:offline-owner:v1', 'user-1')
    await clearProgressQueue('user-1')
    vi.mocked(booksApi.updateProgress).mockReset()
  })

  it('drops a stale update after the API reports a newer revision', async () => {
    await enqueueProgress('book-1', update)
    vi.mocked(booksApi.updateProgress).mockRejectedValue(
      new ApiError(409, { code: 'PROGRESS_CONFLICT', message: 'Newer progress exists' })
    )

    await expect(flushProgressQueue()).resolves.toBe(1)
    await expect(readProgressQueue()).resolves.toEqual([])
  })
})
