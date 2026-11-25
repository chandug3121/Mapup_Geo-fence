import { useState } from 'react'
import { Link } from 'react-router'
import { MapPinned, Plus } from 'lucide-react'
import { errorMessage } from '../api/client'
import type { Vehicle, VehicleLocation } from '../api/types'
import { Button, Card, CategoryBadge, EmptyState, ErrorState, Modal, PageHeader, Skeleton, SkeletonRows } from '../components/ui'
import { VehicleForm } from '../components/vehicles/VehicleForm'
import { useVehicleLocations, useVehicles } from '../hooks/queries'
import { formatDateTime, formatLatLng, timeAgo } from '../lib/format'

export function VehiclesPage() {
  const [registering, setRegistering] = useState(false)
  const vehicles = useVehicles()
  const locations = useVehicleLocations(vehicles.data)

  return (
    <div className="mx-auto max-w-6xl">
      <PageHeader
        title="Vehicles"
        description="Registered vehicles with their latest position and the zones they are in."
        actions={
          <Button icon={<Plus className="size-4" />} onClick={() => setRegistering(true)}>
            Register vehicle
          </Button>
        }
      />
      <div className="px-4 pb-6 sm:px-6">
        {vehicles.isLoading ? (
          <SkeletonRows />
        ) : vehicles.error ? (
          <ErrorState message={errorMessage(vehicles.error)} onRetry={() => void vehicles.refetch()} />
        ) : !vehicles.data?.length ? (
          <Card>
            <EmptyState
              title="No vehicles yet"
              body="Register a vehicle to start tracking it."
              action={<Button onClick={() => setRegistering(true)}>Register vehicle</Button>}
            />
          </Card>
        ) : (
          <Card className="overflow-hidden">
            <ul className="divide-y divide-slate-200 dark:divide-slate-800">
              {vehicles.data.map((v) => (
                <VehicleRow key={v.id} vehicle={v} location={locations.byId.get(v.id)} loading={locations.isLoading} />
              ))}
            </ul>
          </Card>
        )}
      </div>

      <Modal open={registering} title="Register vehicle" onClose={() => setRegistering(false)}>
        <VehicleForm onDone={() => setRegistering(false)} />
      </Modal>
    </div>
  )
}

function VehicleRow({ vehicle, location, loading }: { vehicle: Vehicle; location?: VehicleLocation; loading: boolean }) {
  const pos = location?.current_location
  return (
    <li className="grid gap-3 p-4 md:grid-cols-[1.2fr_1fr_1.4fr_auto] md:items-center">
      <div>
        <p className="font-mono font-semibold">{vehicle.vehicle_number}</p>
        <p className="text-sm text-slate-500">
          {vehicle.driver_name} · <span className="capitalize">{vehicle.vehicle_type}</span> · {vehicle.phone}
        </p>
      </div>
      <div className="text-sm">
        {loading && !location ? (
          <Skeleton className="h-8 w-40" />
        ) : pos ? (
          <>
            <p className="font-mono text-xs">{formatLatLng(pos.latitude, pos.longitude)}</p>
            <p className="text-xs text-slate-500" title={formatDateTime(pos.timestamp)}>
              {timeAgo(pos.timestamp)}
            </p>
          </>
        ) : (
          <p className="text-xs text-slate-500">No location yet</p>
        )}
      </div>
      <div className="flex flex-wrap gap-1.5">
        {location?.current_geofences.length ? (
          location.current_geofences.map((g) => (
            <span key={g.geofence_id} className="inline-flex items-center gap-1 text-xs">
              {g.geofence_name}
              <CategoryBadge category={g.category} />
            </span>
          ))
        ) : (
          pos && <span className="text-xs text-slate-500">Outside all zones</span>
        )}
      </div>
      <Link
        to={`/?vehicle=${encodeURIComponent(vehicle.id)}`}
        className="inline-flex items-center gap-1.5 text-sm font-medium text-brand-600 hover:underline"
      >
        <MapPinned className="size-4" aria-hidden /> Move on map
      </Link>
    </li>
  )
}
