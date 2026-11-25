import { useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import { MapContainer, TileLayer } from 'react-leaflet'
import { PenLine, X } from 'lucide-react'
import { CATEGORIES, type Category, type LatLng } from '../api/types'
import { AlertFeed } from '../components/alerts/AlertFeed'
import { GeofenceForm } from '../components/geofences/GeofenceForm'
import {
  ClickPicker,
  DEFAULT_CENTER,
  DEFAULT_ZOOM,
  DrawPolygon,
  FitOnce,
  FlyTo,
  GeofenceLayer,
  InvalidateOnResize,
  RoutePoints,
  TILE_ATTRIBUTION,
  TILE_URL,
  VehicleMarkers,
} from '../components/map/MapParts'
import { Button, ErrorState, Modal, cx } from '../components/ui'
import { LocationPanel, type MapMode } from '../components/vehicles/LocationPanel'
import { useAlertFeed } from '../context/AlertFeedContext'
import { useGeofences, useVehicleLocations, useVehicles } from '../hooks/queries'
import { CATEGORY_STYLES } from '../lib/categories'
import { errorMessage } from '../api/client'

type Panel = 'move' | 'feed'

export function MapPage() {
  const [params, setParams] = useSearchParams()
  const panel: Panel = params.get('panel') === 'feed' ? 'feed' : 'move'
  const [vehicleId, setVehicleId] = useState(params.get('vehicle') ?? '')
  const [hidden, setHidden] = useState<Set<Category>>(new Set())
  const [mode, setMode] = useState<MapMode>('idle')
  const [pickedPoint, setPickedPoint] = useState<LatLng | null>(null)
  const [route, setRoute] = useState<LatLng[]>([])
  const [drawnRing, setDrawnRing] = useState<LatLng[] | null>(null)
  const [flyTarget, setFlyTarget] = useState<{ point: LatLng; key: number } | null>(null)
  const { unread, markAllRead } = useAlertFeed()

  const geofences = useGeofences()
  const vehicles = useVehicles()
  const locations = useVehicleLocations(vehicles.data)

  const visibleFences = useMemo(() => (geofences.data ?? []).filter((f) => !hidden.has(f.category)), [geofences.data, hidden])

  const fitPoints = useMemo<LatLng[]>(() => {
    const pts: LatLng[] = (geofences.data ?? []).flatMap((f) => f.coordinates)
    for (const loc of locations.byId.values()) {
      if (loc.current_location) pts.push([loc.current_location.latitude, loc.current_location.longitude])
    }
    return pts
  }, [geofences.data, locations.byId])

  const handlePick = useCallback(
    (point: LatLng) => {
      if (mode === 'pick') setPickedPoint(point)
      if (mode === 'route') setRoute((r) => [...r, point])
    },
    [mode],
  )

  const flyTo = useCallback((point: LatLng) => setFlyTarget({ point, key: Date.now() }), [])

  function setPanel(next: Panel) {
    const p = new URLSearchParams(params)
    p.set('panel', next)
    setParams(p, { replace: true })
    if (next === 'feed') markAllRead()
  }

  function toggleCategory(c: Category) {
    setHidden((prev) => {
      const next = new Set(prev)
      if (next.has(c)) next.delete(c)
      else next.add(c)
      return next
    })
  }

  const loadError = geofences.error ?? vehicles.error

  return (
    <div className="flex h-full flex-col lg:flex-row">
      <section className="relative h-[55dvh] min-h-80 shrink-0 lg:h-auto lg:flex-1" aria-label="Map">
        <MapContainer center={DEFAULT_CENTER} zoom={DEFAULT_ZOOM} className="h-full w-full">
          <TileLayer url={TILE_URL} attribution={TILE_ATTRIBUTION} />
          <InvalidateOnResize />
          <FitOnce points={fitPoints} />
          <FlyTo target={flyTarget} />
          <GeofenceLayer fences={visibleFences} />
          <VehicleMarkers vehicles={vehicles.data ?? []} locations={locations.byId} selectedId={vehicleId} onSelect={setVehicleId} />
          <RoutePoints points={route} />
          <ClickPicker active={mode === 'pick' || mode === 'route'} onPick={handlePick} />
          <DrawPolygon
            active={mode === 'draw'}
            onDrawn={(ring) => {
              setMode('idle')
              setDrawnRing(ring)
            }}
          />
        </MapContainer>

        {/* Map toolbar */}
        <div className="pointer-events-none absolute inset-x-0 top-0 z-[400] flex flex-wrap items-start justify-between gap-2 p-3 pl-14">
          <div className="pointer-events-auto flex flex-wrap gap-1.5" role="group" aria-label="Show zone categories">
            {CATEGORIES.map((c) => {
              const style = CATEGORY_STYLES[c]
              const on = !hidden.has(c)
              return (
                <button
                  key={c}
                  type="button"
                  aria-pressed={on}
                  onClick={() => toggleCategory(c)}
                  className={cx(
                    'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium shadow-sm ring-1 transition',
                    on ? 'bg-white text-slate-800 ring-slate-300' : 'bg-white/70 text-slate-400 ring-slate-200 line-through',
                  )}
                >
                  <span className="size-2 rounded-full" style={{ background: on ? style.color : '#cbd5e1' }} aria-hidden />
                  {style.short}
                </button>
              )
            })}
          </div>
          <div className="pointer-events-auto">
            {mode === 'draw' ? (
              <Button size="sm" variant="secondary" icon={<X className="size-4" />} onClick={() => setMode('idle')}>
                Cancel drawing
              </Button>
            ) : (
              <Button size="sm" icon={<PenLine className="size-4" />} onClick={() => setMode('draw')} className="shadow-sm">
                Draw zone
              </Button>
            )}
          </div>
        </div>

        {mode !== 'idle' && (
          <p className="pointer-events-none absolute inset-x-0 bottom-6 z-[400] mx-auto w-fit max-w-[90%] rounded-full bg-slate-900/85 px-4 py-1.5 text-center text-xs text-white">
            {mode === 'draw' && 'Click to add corners. Click the first corner to finish.'}
            {mode === 'pick' && 'Click the map to set the vehicle position.'}
            {mode === 'route' && 'Click the map to add route points, then press Play.'}
          </p>
        )}
      </section>

      <aside className="flex min-h-0 flex-1 flex-col border-slate-200 bg-white lg:w-96 lg:flex-none lg:border-l dark:border-slate-800 dark:bg-slate-950">
        <div role="tablist" className="flex shrink-0 border-b border-slate-200 dark:border-slate-800">
          {(['move', 'feed'] as const).map((p) => (
            <button
              key={p}
              role="tab"
              type="button"
              aria-selected={panel === p}
              onClick={() => setPanel(p)}
              className={cx(
                'flex-1 border-b-2 px-4 py-3 text-sm font-medium',
                panel === p ? 'border-brand-600 text-brand-700 dark:text-white' : 'border-transparent text-slate-500 hover:text-slate-800',
              )}
            >
              {p === 'move' ? 'Move vehicle' : 'Live alerts'}
              {p === 'feed' && unread > 0 && panel !== 'feed' && (
                <span className="ml-1.5 rounded-full bg-red-600 px-1.5 text-[11px] text-white">{unread}</span>
              )}
            </button>
          ))}
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {loadError && <ErrorState message={errorMessage(loadError)} onRetry={() => void Promise.all([geofences.refetch(), vehicles.refetch()])} />}
          {panel === 'move' ? (
            <LocationPanel
              vehicles={vehicles.data ?? []}
              vehicleId={vehicleId}
              onVehicleChange={setVehicleId}
              mode={mode}
              onModeChange={setMode}
              pickedPoint={pickedPoint}
              route={route}
              onRouteChange={setRoute}
            />
          ) : (
            <AlertFeed onLocate={flyTo} />
          )}
        </div>
      </aside>

      <Modal open={drawnRing !== null} title="New geofence" onClose={() => setDrawnRing(null)}>
        {drawnRing && <GeofenceForm drawnRing={drawnRing} onCreated={() => setDrawnRing(null)} onCancel={() => setDrawnRing(null)} />}
      </Modal>
    </div>
  )
}
