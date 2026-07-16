import type { z } from 'zod'
import { apiConfig } from './config'
import { demoRequest } from './demo'
import type { ApiErrorBody, ApiErrorEnvelope } from '../types/api'

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly details?: Record<string, unknown>
  readonly requestId?: string

  constructor(status: number, body: ApiErrorBody) {
    super(body.message)
    this.name = 'ApiError'
    this.status = status
    this.code = body.code
    this.details = body.details
    this.requestId = body.request_id
  }
}

let csrfToken: string | undefined
let refreshPromise: Promise<boolean> | undefined

export function setCsrfToken(token: string | undefined): void {
  csrfToken = token
}

function csrfFromCookie(): string | undefined {
  if (typeof document === 'undefined') return undefined
  const names = ['bookflow_csrf', 'csrf_token', 'XSRF-TOKEN']
  for (const name of names) {
    const match = document.cookie
      .split('; ')
      .find((item) => item.startsWith(`${encodeURIComponent(name)}=`))
    if (match) return decodeURIComponent(match.slice(match.indexOf('=') + 1))
  }
  return undefined
}

async function refreshSession(): Promise<boolean> {
  if (!refreshPromise) {
    const headers = new Headers({ Accept: 'application/json' })
    const token = csrfToken || csrfFromCookie()
    if (token) headers.set('X-CSRF-Token', token)
    refreshPromise = fetch(`${apiConfig.baseUrl}/auth/refresh`, {
      method: 'POST',
      credentials: 'include',
      headers
    })
      .then((response) => {
        const nextCsrf = response.headers.get('X-CSRF-Token')
        if (nextCsrf) setCsrfToken(nextCsrf)
        return response.ok
      })
      .catch(() => false)
      .finally(() => {
        refreshPromise = undefined
      })
  }
  return refreshPromise
}

async function parseError(response: Response): Promise<ApiError> {
  const fallback: ApiErrorBody = {
    code: response.status === 401 ? 'UNAUTHENTICATED' : 'REQUEST_FAILED',
    message: response.statusText || 'Request failed'
  }
  try {
    const value = (await response.json()) as Partial<ApiErrorEnvelope>
    if (value.error?.code && value.error.message) return new ApiError(response.status, value.error)
  } catch {
    // The API may return an empty body for proxy/network failures.
  }
  return new ApiError(response.status, fallback)
}

export interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown
  retryAuth?: boolean
}

async function rawRequest(path: string, options: RequestOptions = {}): Promise<unknown> {
  if (apiConfig.demo) return demoRequest(path, options)

  const method = (options.method || 'GET').toUpperCase()
  const headers = new Headers(options.headers)
  headers.set('Accept', 'application/json')
  let body: BodyInit | undefined

  if (options.body instanceof FormData || options.body instanceof Blob) {
    body = options.body
  } else if (options.body !== undefined) {
    headers.set('Content-Type', 'application/json')
    body = JSON.stringify(options.body)
  }

  if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    const token = csrfToken || csrfFromCookie()
    if (token) headers.set('X-CSRF-Token', token)
  }

  const response = await fetch(`${apiConfig.baseUrl}${path}`, {
    ...options,
    method,
    body,
    headers,
    credentials: 'include'
  })

  const nextCsrf = response.headers.get('X-CSRF-Token')
  if (nextCsrf) setCsrfToken(nextCsrf)

  const authPath = path.startsWith('/auth/')
  if (response.status === 401 && options.retryAuth !== false && !authPath) {
    const refreshed = await refreshSession()
    if (refreshed) return rawRequest(path, { ...options, retryAuth: false })
  }

  if (!response.ok) throw await parseError(response)
  if (response.status === 204) return undefined
  const text = await response.text()
  return text ? (JSON.parse(text) as unknown) : undefined
}

export async function apiRequest<T>(
  path: string,
  schema: z.ZodType<T>,
  options: RequestOptions = {}
): Promise<T> {
  const value = await rawRequest(path, options)
  const parsed = schema.safeParse(value)
  if (!parsed.success) {
    throw new ApiError(502, {
      code: 'INVALID_API_RESPONSE',
      message: 'The server returned data in an unexpected format',
      details: { issues: parsed.error.issues.slice(0, 8) }
    })
  }
  return parsed.data
}

export async function apiRequestVoid(path: string, options: RequestOptions = {}): Promise<void> {
  await rawRequest(path, options)
}

/**
 * Sends a small, CSRF-protected request while the document is being hidden.
 * sendBeacon cannot attach the required CSRF header, so cookie-authenticated
 * endpoints use fetch keepalive instead.
 */
export function apiKeepalive(path: string, body: unknown): void {
  if (apiConfig.demo) return
  const headers = new Headers({
    Accept: 'application/json',
    'Content-Type': 'application/json'
  })
  const token = csrfToken || csrfFromCookie()
  if (token) headers.set('X-CSRF-Token', token)
  void fetch(`${apiConfig.baseUrl}${path}`, {
    method: 'POST',
    credentials: 'include',
    keepalive: true,
    headers,
    body: JSON.stringify(body)
  }).catch(() => undefined)
}

export function queryString(params: Record<string, string | number | boolean | undefined>): string {
  const search = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') search.set(key, String(value))
  })
  const value = search.toString()
  return value ? `?${value}` : ''
}
