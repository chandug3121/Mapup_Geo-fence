import { serverStatus } from './serverStatus'

const rawBase = (import.meta.env.VITE_API_URL as string | undefined) || 'http://localhost:8080'
export const API_URL = rawBase.replace(/\/+$/, '')
const API_KEY = (import.meta.env.VITE_API_KEY as string | undefined) || ''

/** WebSocket URL derived from the API URL (https → wss, http → ws). */
export function alertsSocketUrl(): string {
  const url = new URL(API_URL + '/ws/alerts')
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  if (API_KEY) url.searchParams.set('api_key', API_KEY)
  return url.toString()
}

/** An error the UI can show directly: the backend writes user-readable messages. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

// Free hosting tiers sleep when idle and can take ~1 minute to wake up,
// so the timeout is generous. The "waking up" banner covers the wait.
const TIMEOUT_MS = 75_000
const SLOW_AFTER_MS = 4_000

type Query = Record<string, string | number | undefined | null>

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'
  body?: unknown
  query?: Query
  signal?: AbortSignal
}

export async function request<T>(path: string, { method = 'GET', body, query, signal }: RequestOptions = {}): Promise<T> {
  const url = new URL(API_URL + path)
  for (const [key, value] of Object.entries(query ?? {})) {
    if (value !== undefined && value !== null && value !== '') url.searchParams.set(key, String(value))
  }

  const controller = new AbortController()
  const onAbort = () => controller.abort()
  signal?.addEventListener('abort', onAbort)
  const timeout = setTimeout(() => controller.abort(), TIMEOUT_MS)
  const ticket = serverStatus.begin(SLOW_AFTER_MS)

  try {
    const res = await fetch(url, {
      method,
      signal: controller.signal,
      headers: {
        Accept: 'application/json',
        ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
        ...(API_KEY ? { 'X-API-Key': API_KEY } : {}),
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
    serverStatus.reachable()

    const data = (await res.json().catch(() => null)) as
      | (T & { error?: { code: string; message: string } })
      | null
    if (!res.ok) {
      throw new ApiError(
        res.status,
        data?.error?.code ?? 'http_error',
        data?.error?.message ?? `Request failed with status ${res.status}`,
      )
    }
    return data as T
  } catch (err) {
    if (err instanceof ApiError) throw err
    if (signal?.aborted) throw err
    serverStatus.unreachable()
    if (controller.signal.aborted) {
      throw new ApiError(0, 'timeout', 'The server took too long to respond. Please try again.')
    }
    throw new ApiError(0, 'network_error', "Can't reach the server. Check your connection and try again.")
  } finally {
    clearTimeout(timeout)
    signal?.removeEventListener('abort', onAbort)
    serverStatus.end(ticket)
  }
}

/** Turns any thrown value into a message for the user. */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return 'Something went wrong.'
}
