import { useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { errorMessage } from '../../api/client'
import { useCreateVehicle } from '../../hooks/queries'
import { VEHICLE_TYPES } from '../../lib/categories'
import { Button, Field, InlineError, Input, Select } from '../ui'

type Errors = Partial<Record<'vehicle_number' | 'driver_name' | 'phone', string>>

const PHONE = /^\+?[0-9(][0-9 ()-]{5,19}$/

export function VehicleForm({ onDone }: { onDone: () => void }) {
  const [form, setForm] = useState({ vehicle_number: '', driver_name: '', vehicle_type: 'truck', phone: '' })
  const [errors, setErrors] = useState<Errors>({})
  const createVehicle = useCreateVehicle()

  const set = (field: keyof typeof form) => (e: { target: { value: string } }) => {
    setForm((f) => ({ ...f, [field]: e.target.value }))
    setErrors((errs) => ({ ...errs, [field]: undefined }))
  }

  function validate(): Errors {
    const errs: Errors = {}
    if (!form.vehicle_number.trim()) errs.vehicle_number = 'Enter the registration number.'
    if (!form.driver_name.trim()) errs.driver_name = "Enter the driver's name."
    if (!form.phone.trim()) errs.phone = 'Enter a contact number.'
    else if (!PHONE.test(form.phone.trim())) errs.phone = 'Use digits, optionally starting with +, e.g. +919876543210.'
    return errs
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const errs = validate()
    setErrors(errs)
    if (Object.keys(errs).length) return
    try {
      const created = await createVehicle.mutateAsync(form)
      toast.success(`Vehicle ${created.vehicle_number} registered`)
      onDone()
    } catch {
      // shown inline below
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
      <Field label="Vehicle number" error={errors.vehicle_number}>
        {(p) => <Input {...p} value={form.vehicle_number} onChange={set('vehicle_number')} placeholder="KA-01-AB-1234" autoCapitalize="characters" />}
      </Field>
      <Field label="Driver name" error={errors.driver_name}>
        {(p) => <Input {...p} value={form.driver_name} onChange={set('driver_name')} placeholder="John Doe" autoComplete="name" />}
      </Field>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Vehicle type">
          {(p) => (
            <Select {...p} value={form.vehicle_type} onChange={set('vehicle_type')}>
              {VEHICLE_TYPES.map((t) => (
                <option key={t} value={t}>
                  {t[0]!.toUpperCase() + t.slice(1)}
                </option>
              ))}
            </Select>
          )}
        </Field>
        <Field label="Driver phone" error={errors.phone}>
          {(p) => <Input {...p} type="tel" value={form.phone} onChange={set('phone')} placeholder="+1234567890" autoComplete="tel" />}
        </Field>
      </div>
      <InlineError message={createVehicle.error ? errorMessage(createVehicle.error) : null} />
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onDone} disabled={createVehicle.isPending}>
          Cancel
        </Button>
        <Button type="submit" loading={createVehicle.isPending}>
          Register vehicle
        </Button>
      </div>
    </form>
  )
}
