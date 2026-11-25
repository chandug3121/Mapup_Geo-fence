import { createContext, use, useCallback, useMemo, useState, type ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { keys } from '../api/endpoints'
import type { AlertMessage } from '../api/types'
import { useAlertSocket, type ConnectionState } from '../hooks/useAlertSocket'
import { CATEGORY_STYLES } from '../lib/categories'

const MAX_FEED = 100

interface AlertFeed {
  alerts: AlertMessage[]
  unread: number
  connection: ConnectionState
  markAllRead: () => void
  clear: () => void
}

const AlertFeedContext = createContext<AlertFeed | null>(null)

/**
 * Owns the single WebSocket connection for the whole app. Each alert is
 * added to the feed, shown as a toast, and refreshes the data it affects.
 */
export function AlertFeedProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [alerts, setAlerts] = useState<AlertMessage[]>([])
  const [unread, setUnread] = useState(0)

  const handleAlert = useCallback(
    (alert: AlertMessage) => {
      setAlerts((prev) => (prev.some((a) => a.event_id === alert.event_id) ? prev : [alert, ...prev].slice(0, MAX_FEED)))
      setUnread((n) => n + 1)
      showToast(alert)
      void queryClient.invalidateQueries({ queryKey: keys.vehicleLocation(alert.vehicle.vehicle_id) })
      void queryClient.invalidateQueries({ queryKey: keys.allViolations })
    },
    [queryClient],
  )

  const connection = useAlertSocket(handleAlert)

  const value = useMemo<AlertFeed>(
    () => ({
      alerts,
      unread,
      connection,
      markAllRead: () => setUnread(0),
      clear: () => {
        setAlerts([])
        setUnread(0)
      },
    }),
    [alerts, unread, connection],
  )

  return <AlertFeedContext value={value}>{children}</AlertFeedContext>
}

export function useAlertFeed(): AlertFeed {
  const ctx = use(AlertFeedContext)
  if (!ctx) throw new Error('useAlertFeed must be used inside AlertFeedProvider')
  return ctx
}

export function isCritical(alert: AlertMessage): boolean {
  return alert.event_type === 'entry' && alert.geofence.category === 'restricted_zone'
}

function showToast(alert: AlertMessage) {
  const verb = alert.event_type === 'entry' ? 'entered' : 'left'
  const zone = CATEGORY_STYLES[alert.geofence.category]?.label ?? alert.geofence.category
  const title = `${alert.vehicle.vehicle_number} ${verb} ${alert.geofence.geofence_name}`
  const description = `${zone} · driver ${alert.vehicle.driver_name}`

  if (isCritical(alert)) {
    toast.error(title, { description, duration: 8000, id: alert.event_id })
  } else if (alert.event_type === 'entry') {
    toast.info(title, { description, duration: 4500, id: alert.event_id })
  } else {
    toast(title, { description, duration: 4500, id: alert.event_id })
  }
}
