import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { useBooks } from './hooks'

describe('API query hooks', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('keeps server state in TanStack Query and returns validated books', async () => {
    const fetchMock = vi.fn(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            items: [
              {
                id: 'book-1',
                title: 'Test book',
                author: 'Author',
                format: 'txt',
                language: 'en',
                processing_status: 'ready',
                progress_percent: 0,
                is_favorite: false,
                tags: [],
                added_at: '2026-07-15T00:00:00Z',
                updated_at: '2026-07-15T00:00:00Z'
              }
            ],
            total: 1,
            has_more: false,
            next_offset: 1
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } }
        )
      )
    )
    vi.stubGlobal('fetch', fetchMock)
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    )

    const { result } = renderHook(() => useBooks({ limit: 10 }), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data?.items[0]?.title).toBe('Test book')
    expect(result.current.data?.total_count).toBe(1)
    expect(client.getQueryCache().getAll()).toHaveLength(1)
  })
})
