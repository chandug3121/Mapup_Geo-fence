import { useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { errorMessage } from '../../api/client'
import { CATEGORIES, type Category, type LatLng } from '../../api/types'
import { useConfigureAlert, useCreateGeofence } from '../../hooks/queries'
import { CATEGORY_STYLES } from '../../lib/categories'
import { closeRing, parseRing } from '../../lib/validation'
import { Button, Field, InlineError, Input, Textarea, cx } from '../ui'

interface GeofenceFormProps {
  /** Ring drawn on the map. When absent, the user types coordinates instead. */
  drawnRing?: LatLng[]
  onCreated?: (id: string) => void
  onCancel?: () => void
}

/**
 * Creates a geofence from a drawn polygon or typed coordinates. For a
 * restricted zone it can also create an "alert on entry" rule in one go.
 */
export function GeofenceForm({ drawnRing, onCreated, onCancel }: GeofenceFormProps) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState<Category>('delivery_zone')
  const [coordsText, setCoordsText] = useState(drawnRing ? JSON.stringify(drawnRing) : '')
  const [alertOnEntry, setAlertOnEntry] = useState(true)
  const [touched, setTouched] = useState(false)

  const createGeofence = useCreateGeofence()
  const configureAlert = useConfigureAlert()

  const ringCheck = parseRing(coordsText)
  const nameError = touched && !name.trim() ? 'Give the zone a name.' : undefined
  const coordsError = (touched || coordsText.length > 0) && !ringCheck.ok ? ringCheck.error : undefined
  const busy = createGeofence.isPending || configureAlert.isPending

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setTouched(true)
    if (!name.trim() || !ringCheck.ok) return

    try {
      const created = await createGeofence.mutateAsync({
        name: name.trim(),
        description: description.trim() || undefined,
        category,
        coordinates: ringCheck.ring,
      })
      if (category === 'restricted_zone' && alertOnEntry) {
        try {
          await configureAlert.mutateAsync({ geofence_id: created.id, event_type: 'entry' })
        } catch (err) {
          toast.warning('Zone created, but the alert rule could not be added', { description: errorMessage(err) })
        }
      }
      toast.success(`Geofence “${created.name}” created`)
      onCreated?.(created.id)
    } catch {
      // The error is shown inline from createGeofence.error.
    }
  }

  function closeForMe() {
    const parsed = safeParse(coordsText)
    if (parsed) setCoordsText(JSON.stringify(closeRing(parsed)))
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
      <Field label="Name" error={nameError}>
        {(p) => <Input {...p} value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Airport cargo gate" maxLength={200} />}
      </Field>

      <Field label="Description" hint="Optional">
        {(p) => (
          <Textarea {...p} rows={2} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="What is this zone for?" />
        )}
      </Field>

      <fieldset>
        <legend className="mb-1.5 text-sm font-medium text-slate-700 dark:text-slate-300">Category</legend>
        <div className="grid grid-cols-2 gap-2">
          {CATEGORIES.map((c) => {
            const style = CATEGORY_STYLES[c]
            const selected = c === category
            return (
              <label
                key={c}
                className={cx(
                  'flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm ring-1 transition',
                  selected
                    ? 'bg-slate-900 text-white ring-slate-900 dark:bg-white dark:text-slate-900 dark:ring-white'
                    : 'ring-slate-300 hover:bg-slate-50 dark:ring-slate-700 dark:hover:bg-slate-800',
                )}
              >
                <input type="radio" name="category" value={c} checked={selected} onChange={() => setCategory(c)} className="sr-only" />
                <span className="size-2.5 rounded-full" style={{ background: style.color }} aria-hidden />
                {style.label}
              </label>
            )
          })}
        </div>
      </fieldset>

      {category === 'restricted_zone' && (
        <label className="flex items-start gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-900 ring-1 ring-red-200 dark:bg-red-950 dark:text-red-100 dark:ring-red-900">
          <input type="checkbox" checked={alertOnEntry} onChange={(e) => setAlertOnEntry(e.target.checked)} className="mt-0.5" />
          <span>Alert me when any vehicle enters this zone</span>
        </label>
      )}

      <Field
        label={drawnRing ? 'Coordinates (from your drawing)' : 'Coordinates'}
        hint={drawnRing ? `${drawnRing.length - 1} corners` : 'JSON list of [latitude, longitude] pairs; the last point must repeat the first.'}
        error={coordsError}
      >
        {(p) => (
          <Textarea
            {...p}
            rows={drawnRing ? 3 : 6}
            readOnly={Boolean(drawnRing)}
            value={coordsText}
            onChange={(e) => setCoordsText(e.target.value)}
            placeholder="[[37.7749, -122.4194], [37.7849, -122.4194], [37.7849, -122.4094], [37.7749, -122.4194]]"
            className="font-mono text-xs"
            spellCheck={false}
          />
        )}
      </Field>
      {!drawnRing && coordsError?.includes('repeat the first point') && (
        <Button variant="secondary" size="sm" onClick={closeForMe} className="self-start">
          Close the polygon for me
        </Button>
      )}

      <InlineError message={createGeofence.error ? errorMessage(createGeofence.error) : null} />

      <div className="flex justify-end gap-2">
        {onCancel && (
          <Button variant="ghost" onClick={onCancel} disabled={busy}>
            Cancel
          </Button>
        )}
        <Button type="submit" loading={busy}>
          Create geofence
        </Button>
      </div>
    </form>
  )
}

function safeParse(text: string): LatLng[] | null {
  try {
    const value: unknown = JSON.parse(text)
    if (Array.isArray(value) && value.every((p) => Array.isArray(p) && p.length === 2 && p.every((n) => typeof n === 'number'))) {
      return value as LatLng[]
    }
  } catch {
    /* not JSON yet */
  }
  return null
}
