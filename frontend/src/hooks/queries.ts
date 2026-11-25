import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, keys } from '../api/endpoints'
import type { Category, LocationUpdateInput, NewAlertRule, NewGeofence, NewVehicle, Status, Vehicle, VehicleLocation } from '../api/types'

// Thin hooks over TanStack Query so pages never deal with cache keys or
// invalidation directly.

export function useGeofences(category?: Category) {
  return useQuery({ queryKey: keys.geofences(category), queryFn: () => api.listGeofences(category) })
}

export function useVehicles() {
  return useQuery({ queryKey: keys.vehicles, queryFn: api.listVehicles })
}

/** Current location + geofences for every vehicle, as a map keyed by vehicle id. */
export function useVehicleLocations(vehicles: Vehicle[] | undefined) {
  return useQueries({
    queries: (vehicles ?? []).map((v) => ({
      queryKey: keys.vehicleLocation(v.id),
      queryFn: () => api.getVehicleLocation(v.id),
      staleTime: 15_000,
    })),
    combine: (results) => {
      const byId = new Map<string, VehicleLocation>()
      for (const r of results) if (r.data) byId.set(r.data.vehicle_id, r.data)
      return { byId, isLoading: results.some((r) => r.isLoading) }
    },
  })
}

export function useAlertRules(filters: { geofence_id?: string; vehicle_id?: string } = {}) {
  return useQuery({ queryKey: keys.alertRules(filters), queryFn: () => api.listAlertRules(filters) })
}

export function useCreateGeofence() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: NewGeofence) => api.createGeofence(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.allGeofences }),
  })
}

export function useCreateVehicle() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: NewVehicle) => api.createVehicle(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.vehicles }),
  })
}

export function useUpdateLocation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: LocationUpdateInput) => api.updateLocation(input),
    onSuccess: (_result, input) => {
      void qc.invalidateQueries({ queryKey: keys.vehicleLocation(input.vehicle_id) })
      void qc.invalidateQueries({ queryKey: keys.allViolations })
    },
  })
}

export function useConfigureAlert() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: NewAlertRule) => api.configureAlert(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.allAlertRules }),
  })
}

export function useSetAlertStatus() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: Status }) => api.setAlertStatus(id, status),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.allAlertRules }),
  })
}

export function useDeleteAlert() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.deleteAlert(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.allAlertRules }),
  })
}
