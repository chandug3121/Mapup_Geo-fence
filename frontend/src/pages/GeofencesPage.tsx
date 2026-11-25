import { useState } from 'react'
import { Link } from 'react-router'
import { Plus } from 'lucide-react'
import { errorMessage } from '../api/client'
import { CATEGORIES, type Category } from '../api/types'
import { GeofenceForm } from '../components/geofences/GeofenceForm'
import { Button, Card, CategoryBadge, EmptyState, ErrorState, Field, Modal, PageHeader, Select, SkeletonRows } from '../components/ui'
import { useGeofences } from '../hooks/queries'
import { CATEGORY_STYLES } from '../lib/categories'
import { formatDateTime } from '../lib/format'

export function GeofencesPage() {
  const [category, setCategory] = useState<Category | ''>('')
  const [creating, setCreating] = useState(false)
  const fences = useGeofences(category || undefined)

  return (
    <div className="mx-auto max-w-5xl">
      <PageHeader
        title="Geofences"
        description="Virtual boundaries that trigger entry and exit events. Draw them on the live map, or enter coordinates here."
        actions={
          <Button icon={<Plus className="size-4" />} onClick={() => setCreating(true)}>
            Add by coordinates
          </Button>
        }
      />

      <div className="px-4 sm:px-6">
        <Field label="Category" className="max-w-xs">
          {(p) => (
            <Select {...p} value={category} onChange={(e) => setCategory(e.target.value as Category | '')}>
              <option value="">All categories</option>
              {CATEGORIES.map((c) => (
                <option key={c} value={c}>
                  {CATEGORY_STYLES[c].label}
                </option>
              ))}
            </Select>
          )}
        </Field>
      </div>

      <div className="p-4 sm:p-6">
        {fences.isLoading ? (
          <SkeletonRows />
        ) : fences.error ? (
          <ErrorState message={errorMessage(fences.error)} onRetry={() => void fences.refetch()} />
        ) : !fences.data?.length ? (
          <Card>
            <EmptyState
              title={category ? `No ${CATEGORY_STYLES[category].label.toLowerCase()}s yet` : 'No geofences yet'}
              body="Draw your first zone on the live map, or paste coordinates."
              action={
                <Link to="/" className="text-sm font-medium text-brand-600 underline">
                  Open the live map
                </Link>
              }
            />
          </Card>
        ) : (
          <ul className="grid gap-3 sm:grid-cols-2">
            {fences.data.map((f) => (
              <li key={f.id}>
                <Card className="flex h-full flex-col gap-2 p-4">
                  <div className="flex items-start justify-between gap-2">
                    <h2 className="font-semibold">{f.name}</h2>
                    <CategoryBadge category={f.category} />
                  </div>
                  {f.description && <p className="text-sm text-slate-600 dark:text-slate-400">{f.description}</p>}
                  <dl className="mt-auto grid grid-cols-2 gap-x-3 gap-y-1 pt-2 text-xs text-slate-500">
                    <dt>Corners</dt>
                    <dd className="text-right">{f.coordinates.length - 1}</dd>
                    <dt>Created</dt>
                    <dd className="text-right">{formatDateTime(f.created_at)}</dd>
                    <dt>ID</dt>
                    <dd className="truncate text-right font-mono">{f.id}</dd>
                  </dl>
                </Card>
              </li>
            ))}
          </ul>
        )}
      </div>

      <Modal open={creating} title="Add geofence by coordinates" onClose={() => setCreating(false)}>
        <GeofenceForm onCreated={() => setCreating(false)} onCancel={() => setCreating(false)} />
      </Modal>
    </div>
  )
}
