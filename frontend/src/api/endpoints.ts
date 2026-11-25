import { request } from './client'
import type {
  AlertRule,
  Category,
  Geofence,
  ListResponse,
  LocationUpdateInput,
  LocationUpdateResult,
  NewAlertRule,
  NewGeofence,
  NewVehicle,
  Status,
  Vehicle,
  VehicleLocation,
  Violation,
  ViolationFilters,
} from './types'

// One function per backend endpoint. Paths match the assessment spec exactly.

export const api = {
  listGeofences: (category?: Category) =>
    request<ListResponse & { geofences: Geofence[] }>('/geofences', { query: { category } }).then((r) => r.geofences),

  createGeofence: (input: NewGeofence) =>
    request<{ id: string; name: string; status: Status }>('/geofences', { method: 'POST', body: input }),

  listVehicles: () => request<ListResponse & { vehicles: Vehicle[] }>('/vehicles').then((r) => r.vehicles),

  createVehicle: (input: NewVehicle) =>
    request<{ id: string; vehicle_number: string; status: Status }>('/vehicles', { method: 'POST', body: input }),

  updateLocation: (input: LocationUpdateInput) =>
    request<LocationUpdateResult>('/vehicles/location', { method: 'POST', body: input }),

  getVehicleLocation: (vehicleId: string) =>
    request<VehicleLocation>(`/vehicles/location/${encodeURIComponent(vehicleId)}`),

  listAlertRules: (filters: { geofence_id?: string; vehicle_id?: string } = {}) =>
    request<ListResponse & { alerts: AlertRule[] }>('/alerts', { query: filters }).then((r) => r.alerts),

  configureAlert: (input: NewAlertRule) =>
    request<{ alert_id: string }>('/alerts/configure', { method: 'POST', body: input }),

  setAlertStatus: (alertId: string, status: Status) =>
    request<AlertRule>(`/alerts/${encodeURIComponent(alertId)}`, { method: 'PATCH', body: { status } }),

  deleteAlert: (alertId: string) =>
    request<{ deleted: boolean }>(`/alerts/${encodeURIComponent(alertId)}`, { method: 'DELETE' }),

  violationHistory: (filters: ViolationFilters) =>
    request<ListResponse & { violations: Violation[] }>('/violations/history', { query: { ...filters } }),

  health: () => request<{ status: string; database: string }>('/health'),
}

/** Query keys in one place so invalidation stays consistent. */
export const keys = {
  geofences: (category?: Category) => ['geofences', category ?? 'all'] as const,
  allGeofences: ['geofences'] as const,
  vehicles: ['vehicles'] as const,
  vehicleLocation: (id: string) => ['vehicleLocation', id] as const,
  allVehicleLocations: ['vehicleLocation'] as const,
  alertRules: (filters: object = {}) => ['alertRules', filters] as const,
  allAlertRules: ['alertRules'] as const,
  violations: (filters: ViolationFilters) => ['violations', filters] as const,
  allViolations: ['violations'] as const,
}
