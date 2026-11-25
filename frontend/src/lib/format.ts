import { format, formatDistanceToNowStrict } from 'date-fns'

export function formatDateTime(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : format(d, 'd MMM yyyy, HH:mm:ss')
}

export function timeAgo(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : formatDistanceToNowStrict(d, { addSuffix: true })
}

export function formatCoord(value: number): string {
  return value.toFixed(5)
}

export function formatLatLng(lat: number, lng: number): string {
  return `${formatCoord(lat)}, ${formatCoord(lng)}`
}

/** Value for <input type="datetime-local"> in the user's timezone. */
export function toLocalInputValue(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` +
    `T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  )
}

/** Converts a datetime-local value (local time, no zone) to ISO 8601 UTC. */
export function localInputToIso(value: string): string | undefined {
  if (!value) return undefined
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString()
}
