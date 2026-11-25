import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, MapPin } from 'lucide-react'
import { errorMessage } from '../api/client'
import { api, keys } from '../api/endpoints'
import type { ViolationFilters } from '../api/types'
import { Button, Card, CategoryBadge, EmptyState, ErrorState, EventPill, Field, Input, PageHeader, Select, SkeletonRows, cx } from '../components/ui'
import { useGeofences, useVehicles } from '../hooks/queries'
import { formatDateTime, formatLatLng, localInputToIso } from '../lib/format'

const PAGE_SIZES = [25, 50, 100]

/** Filters live in the URL so a filtered view survives refresh and can be shared. */
export function HistoryPage() {
  const [params, setParams] = useSearchParams()
  const vehicles = useVehicles()
  const geofences = useGeofences()

  const vehicleId = params.get('vehicle_id') ?? ''
  const geofenceId = params.get('geofence_id') ?? ''
  const start = params.get('start') ?? ''
  const end = params.get('end') ?? ''
  const limit = PAGE_SIZES.includes(Number(params.get('limit'))) ? Number(params.get('limit')) : 50
  const offset = Math.max(0, Number(params.get('offset')) || 0)

  const startIso = localInputToIso(start)
  const endIso = localInputToIso(end)
  const rangeError = startIso && endIso && endIso < startIso ? 'The end date must be after the start date.' : null

  const filters = useMemo<ViolationFilters>(
    () => ({ vehicle_id: vehicleId || undefined, geofence_id: geofenceId || undefined, start_date: startIso, end_date: endIso, limit, offset }),
    [vehicleId, geofenceId, startIso, endIso, limit, offset],
  )

  const history = useQuery({
    queryKey: keys.violations(filters),
    queryFn: () => api.violationHistory(filters),
    placeholderData: keepPreviousData,
    enabled: !rangeError,
  })

  function setParam(name: string, value: string) {
    const next = new URLSearchParams(params)
    if (value) next.set(name, value)
    else next.delete(name)
    if (name !== 'offset') next.delete('offset') // new filters start from page 1
    setParams(next, { replace: true })
  }

  const total = history.data?.total_count ?? 0
  const rows = history.data?.violations ?? []
  const from = total === 0 ? 0 : offset + 1
  const to = Math.min(offset + rows.length, total)

  return (
    <div className="mx-auto max-w-6xl">
      <PageHeader title="Violation history" description="Every geofence entry and exit, newest first." />

      <div className="grid gap-3 px-4 sm:grid-cols-2 sm:px-6 lg:grid-cols-5">
        <Field label="Vehicle">
          {(p) => (
            <Select {...p} value={vehicleId} onChange={(e) => setParam('vehicle_id', e.target.value)}>
              <option value="">All vehicles</option>
              {vehicles.data?.map((v) => (
                <option key={v.id} value={v.id}>
                  {v.vehicle_number}
                </option>
              ))}
            </Select>
          )}
        </Field>
        <Field label="Geofence">
          {(p) => (
            <Select {...p} value={geofenceId} onChange={(e) => setParam('geofence_id', e.target.value)}>
              <option value="">All geofences</option>
              {geofences.data?.map((g) => (
                <option key={g.id} value={g.id}>
                  {g.name}
                </option>
              ))}
            </Select>
          )}
        </Field>
        <Field label="From">{(p) => <Input {...p} type="datetime-local" value={start} onChange={(e) => setParam('start', e.target.value)} />}</Field>
        <Field label="To" error={rangeError ?? undefined}>
          {(p) => <Input {...p} type="datetime-local" value={end} onChange={(e) => setParam('end', e.target.value)} />}
        </Field>
        <Field label="Rows per page">
          {(p) => (
            <Select {...p} value={limit} onChange={(e) => setParam('limit', e.target.value)}>
              {PAGE_SIZES.map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </Select>
          )}
        </Field>
      </div>

      <div className="p-4 sm:p-6">
        <Card className="overflow-hidden">
          {history.isLoading ? (
            <SkeletonRows rows={6} />
          ) : history.error ? (
            <ErrorState message={errorMessage(history.error)} onRetry={() => void history.refetch()} />
          ) : rows.length === 0 ? (
            <EmptyState title="No events found" body="Try widening the date range or clearing filters. Events appear when vehicles cross geofence boundaries." />
          ) : (
            <div className={cx('overflow-x-auto transition-opacity', history.isPlaceholderData && 'opacity-60')}>
              <table className="w-full min-w-[640px] text-left text-sm">
                <thead className="bg-slate-50 text-xs text-slate-500 uppercase dark:bg-slate-900">
                  <tr>
                    <th className="px-4 py-2.5 font-medium">Time</th>
                    <th className="px-4 py-2.5 font-medium">Vehicle</th>
                    <th className="px-4 py-2.5 font-medium">Geofence</th>
                    <th className="px-4 py-2.5 font-medium">Event</th>
                    <th className="px-4 py-2.5 font-medium">Location</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                  {rows.map((v) => (
                    <tr key={v.id} className="hover:bg-slate-50 dark:hover:bg-slate-900">
                      <td className="px-4 py-2.5 whitespace-nowrap">{formatDateTime(v.timestamp)}</td>
                      <td className="px-4 py-2.5 font-mono">{v.vehicle_number}</td>
                      <td className="px-4 py-2.5">
                        <span className="mr-2">{v.geofence_name}</span>
                        <CategoryBadge category={v.category} />
                      </td>
                      <td className="px-4 py-2.5">
                        <EventPill type={v.event_type} />
                      </td>
                      <td className="px-4 py-2.5">
                        <Link
                          to={`/?vehicle=${encodeURIComponent(v.vehicle_id)}`}
                          className="inline-flex items-center gap-1 font-mono text-xs text-brand-600 hover:underline"
                        >
                          <MapPin className="size-3.5" aria-hidden />
                          {formatLatLng(v.latitude, v.longitude)}
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <div className="flex items-center justify-between border-t border-slate-200 px-4 py-3 text-sm dark:border-slate-800">
            <span className="text-slate-500">
              {total > 0 ? `Showing ${from}–${to} of ${total}` : 'No results'}
            </span>
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="secondary"
                icon={<ChevronLeft className="size-4" />}
                disabled={offset === 0 || history.isFetching}
                onClick={() => setParam('offset', String(Math.max(0, offset - limit)))}
              >
                Previous
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={offset + limit >= total || history.isFetching}
                onClick={() => setParam('offset', String(offset + limit))}
              >
                Next <ChevronRight className="size-4" />
              </Button>
            </div>
          </div>
        </Card>
      </div>
    </div>
  )
}
