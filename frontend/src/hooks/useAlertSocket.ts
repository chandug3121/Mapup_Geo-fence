import { useEffect, useRef, useState } from 'react'
import { alertsSocketUrl } from '../api/client'
import type { AlertMessage } from '../api/types'

export type ConnectionState = 'connecting' | 'open' | 'reconnecting' | 'offline'

const MIN_RETRY_MS = 1_000
const MAX_RETRY_MS = 30_000
// Small client heartbeat. The server ignores the content, but incoming
// messages keep free hosting tiers from treating the connection as idle.
const HEARTBEAT_MS = 45_000

/** Runtime check so a malformed message can't crash the UI. */
export function isAlertMessage(value: unknown): value is AlertMessage {
  if (typeof value !== 'object' || value === null) return false
  const v = value as Record<string, unknown>
  const vehicle = v.vehicle as Record<string, unknown> | undefined
  const geofence = v.geofence as Record<string, unknown> | undefined
  const location = v.location as Record<string, unknown> | undefined
  return (
    typeof v.event_id === 'string' &&
    (v.event_type === 'entry' || v.event_type === 'exit') &&
    typeof v.timestamp === 'string' &&
    typeof vehicle?.vehicle_id === 'string' &&
    typeof geofence?.geofence_id === 'string' &&
    typeof location?.latitude === 'number' &&
    typeof location?.longitude === 'number'
  )
}

/** Exponential backoff with jitter: ~1s, 2s, 4s … capped at 30s. */
export function retryDelay(attempt: number): number {
  const base = Math.min(MAX_RETRY_MS, MIN_RETRY_MS * 2 ** attempt)
  return base / 2 + Math.random() * (base / 2)
}

/**
 * Keeps a WebSocket open to /ws/alerts, reconnecting automatically, and
 * calls onAlert for every valid alert. Safe under React StrictMode: the
 * cleanup closes the socket and cancels pending timers.
 */
export function useAlertSocket(onAlert: (alert: AlertMessage) => void): ConnectionState {
  const [state, setState] = useState<ConnectionState>('connecting')
  const onAlertRef = useRef(onAlert)

  useEffect(() => {
    onAlertRef.current = onAlert
  }, [onAlert])

  useEffect(() => {
    let socket: WebSocket | null = null
    let attempt = 0
    let retryTimer: ReturnType<typeof setTimeout> | undefined
    let heartbeat: ReturnType<typeof setInterval> | undefined
    let disposed = false

    const scheduleReconnect = () => {
      if (disposed) return
      if (!navigator.onLine) {
        setState('offline')
        return // the 'online' listener reconnects
      }
      setState('reconnecting')
      retryTimer = setTimeout(connect, retryDelay(attempt++))
    }

    function connect() {
      if (disposed) return
      clearTimeout(retryTimer)
      try {
        socket = new WebSocket(alertsSocketUrl())
      } catch {
        scheduleReconnect()
        return
      }

      socket.onopen = () => {
        attempt = 0
        setState('open')
        heartbeat = setInterval(() => {
          if (socket?.readyState === WebSocket.OPEN) socket.send('{"type":"ping"}')
        }, HEARTBEAT_MS)
      }
      socket.onmessage = (event) => {
        try {
          const data: unknown = JSON.parse(String(event.data))
          if (isAlertMessage(data)) onAlertRef.current(data)
        } catch {
          console.warn('Ignoring malformed alert message', event.data)
        }
      }
      socket.onclose = () => {
        clearInterval(heartbeat)
        socket = null
        scheduleReconnect()
      }
      // onerror is always followed by onclose, which handles the retry.
    }

    const handleOnline = () => {
      if (!socket) {
        attempt = 0
        connect()
      }
    }
    const handleOffline = () => setState('offline')

    window.addEventListener('online', handleOnline)
    window.addEventListener('offline', handleOffline)
    connect()

    return () => {
      disposed = true
      clearTimeout(retryTimer)
      clearInterval(heartbeat)
      window.removeEventListener('online', handleOnline)
      window.removeEventListener('offline', handleOffline)
      if (socket) {
        socket.onclose = null
        socket.close()
      }
    }
  }, [])

  return state
}
