import type { Category } from '../api/types'

export interface CategoryStyle {
  label: string
  short: string
  color: string
  /** Leaflet dashArray: restricted zones are dashed so the meaning doesn't rely on colour alone. */
  dash?: string
  fillOpacity: number
  badge: string
}

export const CATEGORY_STYLES: Record<Category, CategoryStyle> = {
  delivery_zone: {
    label: 'Delivery zone',
    short: 'Delivery',
    color: '#2563eb',
    fillOpacity: 0.14,
    badge: 'bg-blue-50 text-blue-800 ring-blue-200 dark:bg-blue-950 dark:text-blue-200 dark:ring-blue-800',
  },
  restricted_zone: {
    label: 'Restricted zone',
    short: 'Restricted',
    color: '#dc2626',
    dash: '8 6',
    fillOpacity: 0.2,
    badge: 'bg-red-50 text-red-800 ring-red-200 dark:bg-red-950 dark:text-red-200 dark:ring-red-800',
  },
  toll_zone: {
    label: 'Toll zone',
    short: 'Toll',
    color: '#d97706',
    dash: '2 6',
    fillOpacity: 0.14,
    badge: 'bg-amber-50 text-amber-800 ring-amber-200 dark:bg-amber-950 dark:text-amber-200 dark:ring-amber-800',
  },
  customer_area: {
    label: 'Customer area',
    short: 'Customer',
    color: '#16a34a',
    fillOpacity: 0.14,
    badge: 'bg-green-50 text-green-800 ring-green-200 dark:bg-green-950 dark:text-green-200 dark:ring-green-800',
  },
}

export const VEHICLE_TYPES = ['truck', 'van', 'car', 'motorcycle', 'bus', 'other'] as const
