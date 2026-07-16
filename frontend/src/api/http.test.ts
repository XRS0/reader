import { authApi, booksApi } from './bookflow'
import type { ReaderPreferences } from '../types/api'

const userEnvelope = {
  user: {
    id: 'user-1',
    email: 'reader@example.com',
    display_name: 'Reader',
    locale: 'en',
    timezone: 'UTC',
    created_at: '2026-07-15T00:00:00Z'
  }
}

describe('API client', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    document.cookie = 'csrf_token=; Max-Age=0; path=/'
  })

  it('uses cookie credentials and validates list responses', async () => {
    const fetchMock = vi.fn((...request: Parameters<typeof fetch>) => {
      void request
      return Promise.resolve(
        new Response(
          JSON.stringify({ items: [], next_cursor: null, has_more: false, total_count: 0 }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' }
          }
        )
      )
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(booksApi.list()).resolves.toMatchObject({ items: [], has_more: false })
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/books'),
      expect.objectContaining({ credentials: 'include' })
    )
  })

  it('adds the CSRF header to a cookie-authenticated mutation', async () => {
    document.cookie = 'csrf_token=test-csrf; path=/'
    const fetchMock = vi.fn((...request: Parameters<typeof fetch>) => {
      void request
      return Promise.resolve(
        new Response(JSON.stringify(userEnvelope), {
          status: 200,
          headers: { 'Content-Type': 'application/json' }
        })
      )
    })
    vi.stubGlobal('fetch', fetchMock)

    await authApi.login({
      email: 'reader@example.com',
      password: 'correct-horse-battery',
      device_key: 'device-1',
      device_name: 'Test browser'
    })
    const request = fetchMock.mock.calls[0]
    const init = request?.[1] as RequestInit
    expect(new Headers(init.headers).get('X-CSRF-Token')).toBe('test-csrf')
    expect(init.credentials).toBe('include')
  })

  it('sends the canonical reader font name accepted by the API', async () => {
    const preferences: ReaderPreferences = {
      theme: 'warm' as const,
      background_color: '#f8f1df',
      text_color: '#302d27',
      accent_color: '#3f6658',
      font_family: 'Georgia' as const,
      font_size: 20,
      font_weight: 400,
      line_height: 1.7,
      letter_spacing: 0,
      content_width: 720,
      page_margin: 40,
      text_align: 'left' as const,
      reading_mode: 'scroll' as const,
      show_progress: true,
      show_remaining_time: true,
      controls_brightness: 0.9
    }
    const fetchMock = vi.fn((...request: Parameters<typeof fetch>) => {
      void request
      return Promise.resolve(
        new Response(JSON.stringify(preferences), {
          status: 200,
          headers: { 'Content-Type': 'application/json' }
        })
      )
    })
    vi.stubGlobal('fetch', fetchMock)

    await booksApi.updatePreferences(preferences, 'book-1')

    const init = fetchMock.mock.calls[0]![1] as RequestInit
    expect(typeof init.body).toBe('string')
    expect(JSON.parse(typeof init.body === 'string' ? init.body : '{}')).toMatchObject({
      font_family: 'Georgia'
    })
  })
})
