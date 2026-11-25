import { useEffect, useMemo, useRef } from 'react'
import L from 'leaflet'
import '@geoman-io/leaflet-geoman-free'
import { Marker, Polygon, Popup, Tooltip, useMap, useMapEvents } from 'react-leaflet'
import type { Geofence, LatLng, Vehicle, VehicleLocation } from '../../api/types'
import { CATEGORY_STYLES } from '../../lib/categories'
import { formatDateTime, formatLatLng } from '../../lib/format'
import { closeRing, roundCoord } from '../../lib/validation'

export const DEFAULT_CENTER: LatLng = [37.7799, -122.4144]
export const DEFAULT_ZOOM = 13
export const TILE_URL = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png'
export const TILE_ATTRIBUTION = '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'

/** Draws geofences as polygons styled by category. */
export function GeofenceLayer({ fences, highlightId }: { fences: Geofence[]; highlightId?: string }) {
  return (
    <>
      {fences.map((f) => {
        const style = CATEGORY_STYLES[f.category]
        const highlighted = f.id === highlightId
        return (
          <Polygon
            key={f.id}
            positions={f.coordinates}
            pathOptions={{
              color: style.color,
              weight: highlighted ? 4 : 2,
              dashArray: style.dash,
              fillColor: style.color,
              fillOpacity: highlighted ? style.fillOpacity + 0.15 : style.fillOpacity,
            }}
          >
            <Tooltip sticky>{f.name}</Tooltip>
            <Popup>
              <div className="min-w-44 space-y-1">
                <p className="font-semibold">{f.name}</p>
                <p className="text-xs" style={{ color: style.color }}>
                  {style.label}
                </p>
                {f.description && <p className="text-xs text-slate-600">{f.description}</p>}
                <p className="font-mono text-[11px] text-slate-500">{f.id}</p>
              </div>
            </Popup>
          </Polygon>
        )
      })}
    </>
  )
}

const TYPE_GLYPH: Record<string, string> = {
  truck: '🚚',
  van: '🚐',
  car: '🚗',
  motorcycle: '🏍️',
  bus: '🚌',
}

function vehicleIcon(v: Vehicle, selected: boolean, inRestricted: boolean): L.DivIcon {
  const classes = ['vehicle-pin', selected && 'vehicle-pin--selected', inRestricted && 'vehicle-pin--alert'].filter(Boolean).join(' ')
  const glyph = TYPE_GLYPH[v.vehicle_type] ?? '📍'
  const label = v.vehicle_number.replace(/[<>&"]/g, '')
  return L.divIcon({
    className: '',
    html: `<div class="${classes}"><div class="vehicle-pin__dot" aria-hidden="true">${glyph}</div><div class="vehicle-pin__label">${label}</div></div>`,
    iconSize: [80, 52],
    iconAnchor: [40, 17],
    popupAnchor: [0, -18],
  })
}

interface VehicleMarkersProps {
  vehicles: Vehicle[]
  locations: Map<string, VehicleLocation>
  selectedId?: string
  onSelect?: (id: string) => void
}

/** One marker per vehicle that has reported a location. */
export function VehicleMarkers({ vehicles, locations, selectedId, onSelect }: VehicleMarkersProps) {
  return (
    <>
      {vehicles.map((v) => {
        const loc = locations.get(v.id)
        const pos = loc?.current_location
        if (!pos) return null
        const inRestricted = loc.current_geofences.some((g) => g.category === 'restricted_zone')
        return (
          <Marker
            key={v.id}
            position={[pos.latitude, pos.longitude]}
            icon={vehicleIcon(v, v.id === selectedId, inRestricted)}
            eventHandlers={{ click: () => onSelect?.(v.id) }}
            zIndexOffset={v.id === selectedId ? 1000 : 0}
          >
            <Popup>
              <div className="min-w-48 space-y-1 text-sm">
                <p className="font-semibold">{v.vehicle_number}</p>
                <p className="text-xs text-slate-600">
                  {v.driver_name} · {v.vehicle_type}
                </p>
                <p className="font-mono text-xs">{formatLatLng(pos.latitude, pos.longitude)}</p>
                <p className="text-xs text-slate-500">{formatDateTime(pos.timestamp)}</p>
                <p className="text-xs">
                  {loc.current_geofences.length
                    ? `Inside: ${loc.current_geofences.map((g) => g.geofence_name).join(', ')}`
                    : 'Not inside any geofence'}
                </p>
              </div>
            </Popup>
          </Marker>
        )
      })}
    </>
  )
}

/**
 * Polygon drawing with Leaflet-Geoman. While `active`, the user clicks to add
 * corners and clicks the first corner to finish; the ring is returned as
 * [lat, lng] pairs (Leaflet's order, same as the API).
 */
export function DrawPolygon({ active, onDrawn }: { active: boolean; onDrawn: (ring: LatLng[]) => void }) {
  const map = useMap()
  const onDrawnRef = useRef(onDrawn)
  onDrawnRef.current = onDrawn

  useEffect(() => {
    const handleCreate = (e: { layer: L.Layer }) => {
      const layer = e.layer as L.Polygon
      const latlngs = layer.getLatLngs()[0] as L.LatLng[]
      map.removeLayer(layer) // the saved geofence is drawn by GeofenceLayer instead
      const ring = closeRing(latlngs.map((p): LatLng => [roundCoord(p.lat), roundCoord(p.lng)]))
      onDrawnRef.current(ring)
    }
    map.on('pm:create', handleCreate)
    return () => {
      map.off('pm:create', handleCreate)
    }
  }, [map])

  useEffect(() => {
    if (active) {
      map.pm.enableDraw('Polygon', {
        allowSelfIntersection: false,
        snappable: true,
        templineStyle: { color: '#2f6fed' },
        hintlineStyle: { color: '#2f6fed', dashArray: '5 5' },
        pathOptions: { color: '#2f6fed', fillOpacity: 0.15 },
      })
    } else {
      map.pm.disableDraw()
    }
    return () => {
      map.pm.disableDraw()
    }
  }, [map, active])

  return null
}

/** Calls onPick with the clicked point while active, with a crosshair cursor. */
export function ClickPicker({ active, onPick }: { active: boolean; onPick: (point: LatLng) => void }) {
  const map = useMapEvents({
    click(e) {
      if (active) onPick([roundCoord(e.latlng.lat), roundCoord(e.latlng.lng)])
    },
  })
  useEffect(() => {
    const el = map.getContainer()
    el.classList.toggle('leaflet-picking', active)
    return () => el.classList.remove('leaflet-picking')
  }, [map, active])
  return null
}

/** Fits the view to the given points once, the first time there are any. */
export function FitOnce({ points }: { points: LatLng[] }) {
  const map = useMap()
  const done = useRef(false)
  const bounds = useMemo(() => (points.length ? L.latLngBounds(points) : null), [points])
  useEffect(() => {
    if (done.current || !bounds) return
    done.current = true
    map.fitBounds(bounds, { padding: [40, 40], maxZoom: 15 })
  }, [map, bounds])
  return null
}

/** Small numbered markers for a planned route. */
export function RoutePoints({ points }: { points: LatLng[] }) {
  return (
    <>
      {points.map((p, i) => (
        <Marker
          key={`${p[0]},${p[1]},${i}`}
          position={p}
          interactive={false}
          icon={L.divIcon({ className: '', html: `<div class="route-point">${i + 1}</div>`, iconSize: [14, 14], iconAnchor: [7, 7] })}
        />
      ))}
    </>
  )
}

/** Lets a parent fly the map to a point by changing `target`. */
export function FlyTo({ target }: { target: { point: LatLng; key: number } | null }) {
  const map = useMap()
  useEffect(() => {
    if (target) map.flyTo(target.point, Math.max(map.getZoom(), 15), { duration: 0.8 })
  }, [map, target])
  return null
}

/** Leaflet needs a size recalculation when its container changes size. */
export function InvalidateOnResize() {
  const map = useMap()
  useEffect(() => {
    const el = map.getContainer()
    const observer = new ResizeObserver(() => map.invalidateSize())
    observer.observe(el)
    return () => observer.disconnect()
  }, [map])
  return null
}
