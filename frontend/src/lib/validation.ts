import type { LatLng } from '../api/types'

/** Result of checking a polygon typed or pasted by the user. */
export type RingCheck = { ok: true; ring: LatLng[] } | { ok: false; error: string }

/**
 * Parses and checks a JSON ring of [lat, lng] pairs with the same rules as
 * the backend, so the user gets instant feedback before submitting.
 */
export function parseRing(text: string): RingCheck {
  if (!text.trim()) return { ok: false, error: 'Enter coordinates as [[lat, lng], …].' }

  let value: unknown
  try {
    value = JSON.parse(text)
  } catch {
    return { ok: false, error: 'That is not valid JSON. Use the format [[37.77, -122.42], [37.78, -122.42], …].' }
  }
  if (!Array.isArray(value)) return { ok: false, error: 'Coordinates must be a list of [lat, lng] pairs.' }

  const ring: LatLng[] = []
  for (const [i, pair] of value.entries()) {
    if (!Array.isArray(pair) || pair.length !== 2 || !pair.every((n) => typeof n === 'number' && Number.isFinite(n))) {
      return { ok: false, error: `Point ${i + 1} must be a [latitude, longitude] pair of numbers.` }
    }
    const [lat, lng] = pair as [number, number]
    if (lat < -90 || lat > 90) return { ok: false, error: `Point ${i + 1}: latitude must be between -90 and 90.` }
    if (lng < -180 || lng > 180) return { ok: false, error: `Point ${i + 1}: longitude must be between -180 and 180.` }
    ring.push([lat, lng])
  }
  return checkRing(ring)
}

export function checkRing(ring: LatLng[]): RingCheck {
  if (ring.length < 4) {
    return { ok: false, error: `A polygon needs at least 4 points (3 corners plus the closing point); you have ${ring.length}.` }
  }
  const first = ring[0]!
  const last = ring[ring.length - 1]!
  if (first[0] !== last[0] || first[1] !== last[1]) {
    return { ok: false, error: 'The last point must repeat the first point to close the polygon.' }
  }
  const unique = new Set(ring.slice(0, -1).map(([lat, lng]) => `${lat},${lng}`))
  if (unique.size < 3) return { ok: false, error: 'A polygon needs at least 3 different corners.' }
  return { ok: true, ring }
}

/** Appends the first point if the ring isn't closed yet. */
export function closeRing(ring: LatLng[]): LatLng[] {
  if (ring.length === 0) return ring
  const first = ring[0]!
  const last = ring[ring.length - 1]!
  return first[0] === last[0] && first[1] === last[1] ? ring : [...ring, [first[0], first[1]]]
}

/** Rounds to 6 decimals (~10 cm), plenty for geofences and easier to read. */
export function roundCoord(n: number): number {
  return Math.round(n * 1e6) / 1e6
}

export function isValidLatitude(n: number): boolean {
  return Number.isFinite(n) && n >= -90 && n <= 90
}

export function isValidLongitude(n: number): boolean {
  return Number.isFinite(n) && n >= -180 && n <= 180
}
