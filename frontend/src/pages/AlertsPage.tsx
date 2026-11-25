import { useState, type FormEvent } from 'react'
import { Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { ApiError, errorMessage } from '../api/client'
import type { AlertRule, Category, RuleEventType } from '../api/types'
import { Button, Card, CategoryBadge, EmptyState, ErrorState, EventPill, Field, InlineError, Modal, PageHeader, Select, SkeletonRows, cx } from '../components/ui'
import { useAlertRules, useConfigureAlert, useDeleteAlert, useGeofences, useSetAlertStatus, useVehicles } from '../hooks/queries'
import { formatDateTime } from '../lib/format'

const EVENT_TYPES: { value: RuleEventType; label: string }[] = [
  { value: 'entry', label: 'Entry' },
  { value: 'exit', label: 'Exit' },
  { value: 'both', label: 'Both' },
]

export function AlertsPage() {
  const [filters, setFilters] = useState({ geofence_id: '', vehicle_id: '' })
  const geofences = useGeofences()
  const vehicles = useVehicles()
  const rules = useAlertRules({
    geofence_id: filters.geofence_id || undefined,
    vehicle_id: filters.vehicle_id || undefined,
  })
  const fenceById = new Map((geofences.data ?? []).map((g) => [g.id, g]))

  return (
    <div className="mx-auto max-w-6xl">
      <PageHeader title="Alert rules" description="Choose which geofence events send a real-time alert to this dashboard." />
      <div className="grid gap-6 px-4 pb-6 sm:px-6 lg:grid-cols-[22rem_1fr]">
        <Card className="h-fit p-4">
          <h2 className="mb-4 font-semibold">New rule</h2>
          <RuleForm />
        </Card>

        <div className="flex flex-col gap-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Filter by geofence">
              {(p) => (
                <Select {...p} value={filters.geofence_id} onChange={(e) => setFilters((f) => ({ ...f, geofence_id: e.target.value }))}>
                  <option value="">All geofences</option>
                  {geofences.data?.map((g) => (
                    <option key={g.id} value={g.id}>
                      {g.name}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
            <Field label="Filter by vehicle">
              {(p) => (
                <Select {...p} value={filters.vehicle_id} onChange={(e) => setFilters((f) => ({ ...f, vehicle_id: e.target.value }))}>
                  <option value="">All vehicles</option>
                  {vehicles.data?.map((v) => (
                    <option key={v.id} value={v.id}>
                      {v.vehicle_number}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
          </div>

          <Card className="overflow-hidden">
            {rules.isLoading ? (
              <SkeletonRows />
            ) : rules.error ? (
              <ErrorState message={errorMessage(rules.error)} onRetry={() => void rules.refetch()} />
            ) : !rules.data?.length ? (
              <EmptyState title="No alert rules" body="Create a rule to be notified when vehicles enter or leave a zone." />
            ) : (
              <ul className="divide-y divide-slate-200 dark:divide-slate-800">
                {rules.data.map((r) => (
                  <RuleRow key={r.alert_id} rule={r} category={fenceById.get(r.geofence_id)?.category} />
                ))}
              </ul>
            )}
          </Card>
        </div>
      </div>
    </div>
  )
}

function RuleForm() {
  const geofences = useGeofences()
  const vehicles = useVehicles()
  const configure = useConfigureAlert()
  const [geofenceId, setGeofenceId] = useState('')
  const [vehicleId, setVehicleId] = useState('')
  const [eventType, setEventType] = useState<RuleEventType>('entry')
  const [touched, setTouched] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setTouched(true)
    if (!geofenceId) return
    try {
      await configure.mutateAsync({ geofence_id: geofenceId, vehicle_id: vehicleId || undefined, event_type: eventType })
      toast.success('Alert rule created')
      setTouched(false)
    } catch {
      // shown inline
    }
  }

  const conflict = configure.error instanceof ApiError && configure.error.status === 409
  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
      <Field label="Geofence" error={touched && !geofenceId ? 'Choose a geofence to watch.' : undefined}>
        {(p) => (
          <Select {...p} value={geofenceId} onChange={(e) => setGeofenceId(e.target.value)} disabled={geofences.isLoading}>
            <option value="">{geofences.isLoading ? 'Loading…' : 'Choose a geofence…'}</option>
            {geofences.data?.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name} ({g.category.replace('_', ' ')})
              </option>
            ))}
          </Select>
        )}
      </Field>
      <Field label="Vehicle" hint="Leave as “All vehicles” to watch every vehicle.">
        {(p) => (
          <Select {...p} value={vehicleId} onChange={(e) => setVehicleId(e.target.value)} disabled={vehicles.isLoading}>
            <option value="">All vehicles</option>
            {vehicles.data?.map((v) => (
              <option key={v.id} value={v.id}>
                {v.vehicle_number} — {v.driver_name}
              </option>
            ))}
          </Select>
        )}
      </Field>
      <fieldset>
        <legend className="mb-1.5 text-sm font-medium text-slate-700 dark:text-slate-300">Alert on</legend>
        <div className="grid grid-cols-3 rounded-lg bg-slate-100 p-1 dark:bg-slate-800">
          {EVENT_TYPES.map((t) => (
            <label
              key={t.value}
              className={cx(
                'cursor-pointer rounded-md py-1.5 text-center text-sm font-medium',
                eventType === t.value ? 'bg-white shadow-sm dark:bg-slate-950' : 'text-slate-500',
              )}
            >
              <input type="radio" name="event_type" className="sr-only" checked={eventType === t.value} onChange={() => setEventType(t.value)} />
              {t.label}
            </label>
          ))}
        </div>
      </fieldset>
      <InlineError message={conflict ? 'This rule already exists.' : configure.error ? errorMessage(configure.error) : null} />
      <Button type="submit" loading={configure.isPending}>
        Create rule
      </Button>
    </form>
  )
}

function RuleRow({ rule, category }: { rule: AlertRule; category?: Category }) {
  const setStatus = useSetAlertStatus()
  const remove = useDeleteAlert()
  const [confirming, setConfirming] = useState(false)
  const active = rule.status === 'active'

  async function toggle() {
    try {
      await setStatus.mutateAsync({ id: rule.alert_id, status: active ? 'inactive' : 'active' })
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  async function confirmDelete() {
    try {
      await remove.mutateAsync(rule.alert_id)
      toast.success('Alert rule deleted')
      setConfirming(false)
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  return (
    <li className={cx('flex flex-wrap items-center gap-3 p-4', !active && 'opacity-60')}>
      <div className="min-w-0 flex-1 space-y-1">
        <p className="flex flex-wrap items-center gap-2 font-medium">
          {rule.geofence_name}
          {category && <CategoryBadge category={category} />}
        </p>
        <p className="flex flex-wrap items-center gap-2 text-sm text-slate-500">
          <EventPill type={rule.event_type} />
          <span>{rule.vehicle_number ?? 'All vehicles'}</span>
          <span className="text-xs">· created {formatDateTime(rule.created_at)}</span>
        </p>
      </div>
      <label className="flex items-center gap-2 text-sm">
        <span className="text-slate-500">{active ? 'On' : 'Off'}</span>
        <button
          type="button"
          role="switch"
          aria-checked={active}
          aria-label={`Turn rule ${active ? 'off' : 'on'}`}
          onClick={toggle}
          disabled={setStatus.isPending}
          className={cx('relative h-6 w-11 rounded-full transition', active ? 'bg-brand-600' : 'bg-slate-300 dark:bg-slate-700')}
        >
          <span className={cx('absolute top-0.5 left-0.5 size-5 rounded-full bg-white shadow transition', active && 'translate-x-5')} />
        </button>
      </label>
      <Button size="sm" variant="ghost" icon={<Trash2 className="size-4" />} onClick={() => setConfirming(true)} aria-label="Delete rule" />
      <Modal open={confirming} title="Delete this alert rule?" onClose={() => setConfirming(false)}>
        <p className="mb-4 text-sm text-slate-600 dark:text-slate-400">
          You will stop getting {rule.event_type === 'both' ? 'entry and exit' : rule.event_type} alerts for {rule.geofence_name}. Past history is kept.
        </p>
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={() => setConfirming(false)}>
            Keep it
          </Button>
          <Button variant="danger" loading={remove.isPending} onClick={confirmDelete}>
            Delete rule
          </Button>
        </div>
      </Modal>
    </li>
  )
}
