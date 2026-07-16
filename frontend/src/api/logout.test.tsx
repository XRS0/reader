import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import { get } from 'idb-keyval'
import type { ReactNode } from 'react'
import { useLogout } from './hooks'
import { enqueueProgress } from './offlineQueue'

const progress = {
  chapter_id: 'chapter-1',
  locator_type: 'chapter_offset' as const,
  locator: 'chapter:1:offset:10',
  character_offset: 10,
  progress_percent: 4,
  scroll_percent: 12,
  revision: 1,
  client_id: 'test-client',
  client_timestamp: '2026-07-15T00:00:00Z'
}

describe('logout privacy cleanup', () => {
  it('removes the account progress queue and cached private chapters', async () => {
    localStorage.setItem('bookflow:offline-owner:v1', 'user-1')
    await enqueueProgress('book-1', progress)
    const deleteCache = vi.fn(() => Promise.resolve(true))
    vi.stubGlobal('caches', {
      keys: () => Promise.resolve(['workbox-precache', 'bookflow-last-chapters']),
      delete: deleteCache
    })
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.resolve(new Response(null, { status: 204 })))
    )
    const client = new QueryClient()
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    )
    const { result } = renderHook(() => useLogout(), { wrapper })

    await act(async () => await result.current.mutateAsync())

    expect(localStorage.getItem('bookflow:offline-owner:v1')).toBeNull()
    expect(await get('bookflow:progress-queue:v1:user-1')).toBeUndefined()
    expect(deleteCache).toHaveBeenCalledWith('bookflow-last-chapters')
  })
})
