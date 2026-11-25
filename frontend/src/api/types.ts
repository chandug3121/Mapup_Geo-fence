// Types mirror the backend's JSON. Coordinates are always [latitude, longitude].

export type LatLng = [number, number]

export const CATEGORIES = ['delivery_zone', 'restricted_zone', 'toll_zone', 'customer_area'] as const
export type Category = (typeof CATEGORIES)[number]

export type EventType = 'entry' | 'exit'
export type RuleEventType = EventType | 'both'
export type Status = 'active' | 'inactive'

export interface Geofence {
  id: string
  name: string
  description: string
  coordinates: LatLng[]
  category: Category
  status: Status
  created_at: string
}

export interface NewGeofence {
  name: string
  description?: string
  coordinates: LatLng[]
  category: Category
}

export interface Vehicle {
  id: string
  vehicle_number: string
  driver_name: string
  vehicle_type: string
  phone: string
  status: Status
  created_at: string
}

export interface NewVehicle {
  vehicle_number: string
  driver_name: string
  vehicle_type: string
  phone: string
}

export interface Position {
  latitude: number
  longitude: number
  timestamp: string
}

export interface FenceSummary {
  geofence_id: string
  geofence_name: string
  category: Category
}

export interface VehicleLocation {
  vehicle_id: string
  vehicle_number: string
  current_location: Position | null
  current_geofences: FenceSummary[]
}

export interface LocationUpdateInput {
  vehicle_id: string
  latitude: number
  longitude: number
  timestamp: string
}

export interface LocationUpdateResult {
  vehicle_id: string
  location_updated: boolean
  current_geofences: (FenceSummary & { status: 'inside' })[]
  events: { geofence_id: string; geofence_name: string; event_type: EventType }[]
  alerts_triggered: number
  stale: boolean
}

export interface AlertRule {
  alert_id: string
  geofence_id: string
  geofence_name: string
  vehicle_id: string | null
  vehicle_number: string | null
  event_type: RuleEventType
  status: Status
  created_at: string
}

export interface NewAlertRule {
  geofence_id: string
  vehicle_id?: string
  event_type: RuleEventType
}

export interface Violation {
  id: string
  vehicle_id: string
  vehicle_number: string
  geofence_id: string
  geofence_name: string
  category: Category
  event_type: EventType
  latitude: number
  longitude: number
  timestamp: string
}

export interface ViolationFilters {
  vehicle_id?: string
  geofence_id?: string
  start_date?: string
  end_date?: string
  limit: number
  offset: number
}

/** The exact message the backend pushes over /ws/alerts. */
export interface AlertMessage {
  event_id: string
  event_type: EventType
  timestamp: string
  vehicle: { vehicle_id: string; vehicle_number: string; driver_name: string }
  geofence: { geofence_id: string; geofence_name: string; category: Category }
  location: { latitude: number; longitude: number }
}

export interface ListResponse {
  total_count: number
  time_ns: string
}
