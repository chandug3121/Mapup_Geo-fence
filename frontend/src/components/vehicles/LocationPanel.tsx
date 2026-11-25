import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Crosshair, MapPin, Play, Route, Square, Trash2 } from 'lucide-react'
import { errorMessage } from '../../api/client'
import type { LatLng, LocationUpdateResult, Vehicle } from '../../api/types'
import { useUpdateLocation } from '../../hooks/queries'
import { localInputToIso, toLocalInputValue } from '../../lib/format'
import { isValidLatitude, isValidLongitude } from '../../lib/validation'
import { Button, CategoryBadge, Field, InlineError, Input, Select, cx } from '../ui'

export type MapMode = 'idle' | 'pick' | 'route' | 'draw'

interface LocationPanelProps {
  vehicles: Vehicle[]
  vehicleId: string
  onVehicleChange: (id: string) => void
  mode: MapMode
  onModeChange: (mode: MapMode) => void
  pickedPoint: LatLng | null
  route: LatLng[]
  onRouteChange: (route: LatLng[]) => void
}

const ROUTE_STEP_MS = 1500

/**
 * Sends location updates: type coordinates, click the map, or plan a route
 * and play it back (one update every 1.5 s) to watch entries and exits live.
 */
export function LocationPanel(props: LocationPanelProps) {
  const { vehicles, vehicleId, onVehicleChange, mode, onModeChange, pickedPoint, route, onRouteChange } = props
  const [lat, setLat] = useState('')
  const [lng, setLng] = useState('')
  const [time, setTime] = useState(() => toLocalInputValue(new Date()))
  const [useNow, setUseNow] = useState(true)
  const [result, setResult] = useState<LocationUpdateResult | null>(null)
  const [playing, setPlaying] = useState<number | null>(null) // index of the route step being sent
  const stopRef = useRef(false)
  const update = useUpdateLocation()

  // A point clicked on the map fills the form.
  useEffect(() => {
    if (pickedPoint) {
      setLat(String(pickedPoint[0]))
      setLng(String(pickedPoint[1]))
    }
  }, [pickedPoint])

  useEffect(() => () => void (stopRef.current = true), [])

  const latNum = Number(lat)
  const lngNum = Number(lng)
  const latError = lat !== '' && !isValidLatitude(latNum) ? 'Latitude must be between -90 and 90.' : undefined
  const lngError = lng !== '' && !isValidLongitude(lngNum) ? 'Longitude must be between -180 and 180.' : undefined
  const canSend = Boolean(vehicleId) && lat !== '' && lng !== '' && !latError && !lngError

  async function send(point: LatLng, timestamp: string) {
    const res = await update.mutateAsync({ vehicle_id: vehicleId, latitude: point[0], longitude: point[1], timestamp })
    setResult(res)
    return res
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!canSend) return
    const timestamp = useNow ? new Date().toISOString() : localInputToIso(time)
    if (!timestamp) return
    try {
      await send([latNum, lngNum], timestamp)
      if (mode === 'pick') onModeChange('idle')
    } catch {
      // shown inline
    }
  }

  async function playRoute() {
    if (!vehicleId || route.length === 0) return
    onModeChange('idle')
    stopRef.current = false
    for (let i = 0; i < route.length && !stopRef.current; i++) {
      setPlaying(i)
      try {
        await send(route[i]!, new Date().toISOString())
      } catch {
        break
      }
      if (i < route.length - 1) await new Promise((r) => setTimeout(r, ROUTE_STEP_MS))
    }
    setPlaying(null)
  }

  if (vehicles.length === 0) {
    return <p className="p-4 text-sm text-slate-500">Register a vehicle first (Vehicles page), then come back to move it around.</p>
  }

  return (
    <div className="flex flex-col gap-5 p-4">
      <Field label="Vehicle">
        {(p) => (
          <Select {...p} value={vehicleId} onChange={(e) => onVehicleChange(e.target.value)}>
            <option value="">Choose a vehicle…</option>
            {vehicles.map((v) => (
              <option key={v.id} value={v.id}>
                {v.vehicle_number} — {v.driver_name}
              </option>
            ))}
          </Select>
        )}
      </Field>

      <form onSubmit={handleSubmit} className="flex flex-col gap-3" noValidate>
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold">Single update</h3>
          <Button
            size="sm"
            variant={mode === 'pick' ? 'primary' : 'secondary'}
            icon={<Crosshair className="size-4" />}
            onClick={() => onModeChange(mode === 'pick' ? 'idle' : 'pick')}
            aria-pressed={mode === 'pick'}
          >
            {mode === 'pick' ? 'Click the map…' : 'Pick on map'}
          </Button>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Latitude" error={latError}>
            {(p) => <Input {...p} inputMode="decimal" value={lat} onChange={(e) => setLat(e.target.value)} placeholder="37.7801" className="font-mono" />}
          </Field>
          <Field label="Longitude" error={lngError}>
            {(p) => <Input {...p} inputMode="decimal" value={lng} onChange={(e) => setLng(e.target.value)} placeholder="-122.4150" className="font-mono" />}
          </Field>
        </div>
        <div className="flex flex-col gap-2">
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={useNow} onChange={(e) => setUseNow(e.target.checked)} />
            Use the current time
          </label>
          {!useNow && (
            <Field label="Timestamp" hint="Your local time; sent to the server in UTC.">
              {(p) => <Input {...p} type="datetime-local" step="1" value={time} onChange={(e) => setTime(e.target.value)} />}
            </Field>
          )}
        </div>
        <Button type="submit" disabled={!canSend || playing !== null} loading={update.isPending && playing === null} icon={<MapPin className="size-4" />}>
          Update location
        </Button>
      </form>

      <div className="flex flex-col gap-3 border-t border-slate-200 pt-4 dark:border-slate-800">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold">Simulate a route</h3>
          <span className="text-xs text-slate-500">{route.length} points</span>
        </div>
        <p className="text-xs text-slate-500 dark:text-slate-400">
          Plan points on the map, then play them back one every {ROUTE_STEP_MS / 1000}s to see entries and exits arrive live.
        </p>
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            variant={mode === 'route' ? 'primary' : 'secondary'}
            icon={<Route className="size-4" />}
            onClick={() => onModeChange(mode === 'route' ? 'idle' : 'route')}
            disabled={playing !== null}
            aria-pressed={mode === 'route'}
          >
            {mode === 'route' ? 'Done planning' : 'Plan on map'}
          </Button>
          {playing === null ? (
            <Button size="sm" icon={<Play className="size-4" />} onClick={playRoute} disabled={!vehicleId || route.length === 0}>
              Play
            </Button>
          ) : (
            <Button size="sm" variant="danger" icon={<Square className="size-4" />} onClick={() => (stopRef.current = true)}>
              Stop ({playing + 1}/{route.length})
            </Button>
          )}
          <Button size="sm" variant="ghost" icon={<Trash2 className="size-4" />} onClick={() => onRouteChange([])} disabled={!route.length || playing !== null}>
            Clear
          </Button>
        </div>
      </div>

      <InlineError message={update.error ? errorMessage(update.error) : null} />
      {result && <UpdateResult result={result} />}
    </div>
  )
}

function UpdateResult({ result }: { result: LocationUpdateResult }) {
  return (
    <div className={cx('rounded-lg p-3 text-sm ring-1', 'bg-slate-50 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800')} aria-live="polite">
      <p className="font-medium">Location updated</p>
      {result.stale && (
        <p className="mt-1 text-xs text-amber-700 dark:text-amber-400">
          This timestamp is older than the vehicle's latest update, so it was saved as history only.
        </p>
      )}
      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        {result.current_geofences.length === 0 ? (
          <span className="text-slate-500">Not inside any geofence</span>
        ) : (
          <>
            <span className="text-slate-500">Inside:</span>
            {result.current_geofences.map((g) => (
              <span key={g.geofence_id} className="inline-flex items-center gap-1">
                <span className="font-medium">{g.geofence_name}</span>
                <CategoryBadge category={g.category} />
              </span>
            ))}
          </>
        )}
      </div>
      {result.events.length > 0 && (
        <ul className="mt-2 space-y-0.5 text-xs">
          {result.events.map((ev) => (
            <li key={ev.geofence_id + ev.event_type}>
              {ev.event_type === 'entry' ? '→ Entered' : '← Left'} {ev.geofence_name}
            </li>
          ))}
          <li className="text-slate-500">
            {result.alerts_triggered} alert{result.alerts_triggered === 1 ? '' : 's'} sent
          </li>
        </ul>
      )}
    </div>
  )
}
