import { ArrowLeftFromLine, ArrowRightToLine, ShieldAlert } from 'lucide-react'
import type { AlertMessage, LatLng } from '../../api/types'
import { isCritical, useAlertFeed } from '../../context/AlertFeedContext'
import { formatDateTime, formatLatLng, timeAgo } from '../../lib/format'
import { Button, CategoryBadge, EmptyState, cx } from '../ui'

/** Live list of alerts received over the WebSocket, newest first. */
export function AlertFeed({ onLocate }: { onLocate?: (point: LatLng) => void }) {
  const { alerts, clear } = useAlertFeed()

  if (alerts.length === 0) {
    return (
      <EmptyState
        title="Waiting for alerts"
        body="Alerts appear here the moment a vehicle enters or exits a zone with an alert rule."
      />
    )
  }

  return (
    <div className="flex flex-col">
      <div className="flex items-center justify-between px-4 py-2 text-xs text-slate-500">
        <span>{alerts.length} received this session</span>
        <Button size="sm" variant="ghost" onClick={clear}>
          Clear
        </Button>
      </div>
      <ul className="divide-y divide-slate-200 dark:divide-slate-800" aria-live="polite">
        {alerts.map((a) => (
          <AlertItem key={a.event_id} alert={a} onLocate={onLocate} />
        ))}
      </ul>
    </div>
  )
}

function AlertItem({ alert, onLocate }: { alert: AlertMessage; onLocate?: (point: LatLng) => void }) {
  const critical = isCritical(alert)
  const Icon = critical ? ShieldAlert : alert.event_type === 'entry' ? ArrowRightToLine : ArrowLeftFromLine
  return (
    <li>
      <button
        type="button"
        onClick={() => onLocate?.([alert.location.latitude, alert.location.longitude])}
        className={cx(
          'flex w-full gap-3 px-4 py-3 text-left hover:bg-slate-50 dark:hover:bg-slate-900',
          critical && 'border-l-4 border-red-600 bg-red-50/60 dark:bg-red-950/40',
        )}
      >
        <Icon className={cx('mt-0.5 size-5 shrink-0', critical ? 'text-red-600' : 'text-slate-500')} aria-hidden />
        <div className="min-w-0 flex-1 space-y-1">
          <p className="text-sm">
            <span className="font-mono font-semibold">{alert.vehicle.vehicle_number}</span>{' '}
            {alert.event_type === 'entry' ? 'entered' : 'left'} <span className="font-medium">{alert.geofence.geofence_name}</span>
          </p>
          <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500">
            <CategoryBadge category={alert.geofence.category} />
            <span>{alert.vehicle.driver_name}</span>
            <time dateTime={alert.timestamp} title={formatDateTime(alert.timestamp)}>
              {timeAgo(alert.timestamp)}
            </time>
          </div>
          <p className="font-mono text-[11px] text-slate-400">{formatLatLng(alert.location.latitude, alert.location.longitude)}</p>
        </div>
      </button>
    </li>
  )
}
