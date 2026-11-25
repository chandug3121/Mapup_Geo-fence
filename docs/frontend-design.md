# Frontend Design — Geofence Control Center

## 1. Goals

1. **Map-first.** Everything spatial (geofences, vehicles, location updates) happens on one live map.
2. **Alerts you can't miss.** A restricted-zone entry should be obvious within a second.
3. **Never a blank or frozen screen.** Every request has a loading, empty, and error state.
4. **Works on a phone.** Same features at 375px width, just rearranged.

## 2. Stack

| Concern | Choice |
|---|---|
| Build | Vite + React 18 + TypeScript (strict) |
| Routing | `react-router-dom` v6 |
| Server state | `@tanstack/react-query` v5 |
| Map | `leaflet`, `react-leaflet`, `@geoman-io/leaflet-geoman-free` (polygon drawing), OpenStreetMap tiles |
| Styling | Tailwind CSS |
| Toasts | `sonner` |
| Icons | `lucide-react` |
| Dates | `date-fns` |

Leaflet uses `[lat, lng]` — the same order as the API, so no swapping on the frontend.

## 3. Information architecture

| Route | Screen | Purpose |
|---|---|---|
| `/` | **Live map** (dashboard) | Geofences + vehicles on the map, draw fences, move vehicles, live alert feed |
| `/geofences` | Geofences | List + category filter, create via coordinates |
| `/vehicles` | Vehicles | Register, list with current location & zones |
| `/alerts` | Alert rules | Configure, list, enable/disable, delete |
| `/history` | Violation history | Filterable, paginated table of entry/exit events |

App shell:
- **Desktop (≥ 1024px):** left sidebar nav (icons + labels), main content, top bar with connection status and alert bell.
- **Tablet (640–1023px):** collapsed icon-only sidebar.
- **Mobile (< 640px):** top bar + bottom tab bar (5 tabs); side panels become bottom sheets.

## 4. Visual design

### Tone
Calm, operational "control room" UI: neutral light surfaces, color reserved for meaning (zone category and alert severity). Dark mode via `prefers-color-scheme`.

### Color tokens (Tailwind theme extension)

| Token | Light | Use |
|---|---|---|
| `surface` | `#F8FAFC` | App background |
| `panel` | `#FFFFFF` | Cards, panels |
| `border` | `#E2E8F0` | Hairlines |
| `ink` | `#0F172A` | Primary text |
| `ink-muted` | `#64748B` | Secondary text |
| `brand` | `#4F46E5` | Primary buttons, active nav |
| `danger` | `#DC2626` | Restricted zones, entry alerts, errors |
| `success` | `#16A34A` | Connected, exit from restricted |

### Category colors (map polygons, badges, legend)

| Category | Color | Map fill / stroke |
|---|---|---|
| `delivery_zone` | Blue `#2563EB` | 15% fill, solid 2px stroke |
| `restricted_zone` | Red `#DC2626` | 20% fill, **dashed** 2px stroke (shape cue, not only color) |
| `toll_zone` | Amber `#D97706` | 15% fill, dotted stroke |
| `customer_area` | Green `#16A34A` | 15% fill, solid stroke |

### Type & spacing
- Font: Inter (Google Fonts), fallback system-ui. Sizes 12 / 14 / 16 / 20 / 24. Weights 400 / 500 / 600.
- 4px spacing scale; card padding 16px; radius 8px (controls) / 12px (cards).
- Numbers (coordinates, timestamps) in `tabular-nums`.

## 5. Screens

### 5.1 Live map (`/`)

```
┌──────────────────────────────────────────────────────────────┐
│ Top bar: title · ● Live (green) / ◌ Reconnecting · 🔔 3      │
├────────┬──────────────────────────────────────┬──────────────┤
│  Nav   │  Map                                 │ Side panel   │
│        │  [category chips]      [Draw zone]   │ (tabs)       │
│        │                                      │ • Vehicles   │
│        │   polygons + vehicle markers         │ • Move       │
│        │                                      │ • Live feed  │
│        │  legend (bottom-left)                │              │
└────────┴──────────────────────────────────────┴──────────────┘
```

**Map layer**
- Polygons colored by category, tooltip with name, click → popup (name, category badge, description, alert rule count, "Add alert rule" link).
- Vehicle markers: circular `divIcon` with vehicle-type icon (truck/car/van/bike) and vehicle number label. Red pulsing ring when inside any restricted zone.
- Category filter chips above the map (multi-select, "All" default) — also drives `GET /geofences?category=` on the list page.
- "Fit to data" button.

**Draw geofence**
1. Click **Draw zone** → Geoman polygon mode; helper text "Click to add points, click the first point to finish".
2. On finish → panel opens with form: name, description, category (segmented control), coordinates preview (read-only, collapsible).
3. If category = `restricted_zone`, a checkbox **"Alert me when any vehicle enters"** is pre-checked → after creating the fence, also calls `POST /alerts/configure` (`event_type: entry`, no vehicle).
4. Submit → spinner on button → success toast → polygon becomes permanent; on error the drawn shape stays and the error shows inline.

**Move vehicle (location update)**
- Select vehicle (searchable dropdown).
- Toggle **"Click map to set position"** → cursor becomes crosshair; click fills lat/lng.
- Or type lat/lng manually. Timestamp defaults to now (editable, `datetime-local`).
- Submit → result card: "Inside: Downtown Delivery Zone (delivery), Airport (restricted)" as category-colored chips, or "Not inside any geofence".
- Marker animates to the new position.
- Bonus demo helper: **"Simulate route"** — click several points, sends updates every 1.5s so the video shows entries/exits happening live.

**Live feed tab**
- Newest alerts first (kept in memory, last 100).
- Each item: event icon (→ entry / ← exit), vehicle number + driver, geofence name + category badge, relative time ("12s ago", absolute on hover), coordinates. Click → map flies to location.
- Restricted entries get a red left border.

### 5.2 Geofences (`/geofences`)
- Category filter (select), list of cards: name, category badge, description, point count, created date, mini "View on map" link.
- **Create by coordinates** form: textarea accepting JSON `[[lat,lng],…]`, live validation (closed ring, ≥ 4 points, ranges) with a small preview map before submit. "Close polygon for me" button appends the first point.

### 5.3 Vehicles (`/vehicles`)
- **Register vehicle** button → modal: vehicle number, driver name, type (select: truck, car, van, motorcycle, other), phone.
- Table (cards on mobile): number, driver, type, phone, status, **last location** (lat, lng, time ago), **current zones** (chips). Location/zones from `GET /vehicles/location/{id}` per row (query per vehicle, cached; refreshed when an alert for that vehicle arrives).
- Row action: "Move on map" → navigates to `/` with the vehicle preselected.

### 5.4 Alert rules (`/alerts`)
- Create form: geofence (select, shows category), vehicle (select, first option **All vehicles**), event type (segmented: Entry · Exit · Both).
- Filters: geofence, vehicle.
- List: geofence name + badge, vehicle number or "All vehicles", event type, status toggle (active/inactive), delete (confirm dialog).
- 409 → inline message "This rule already exists".

### 5.5 Violation history (`/history`)
- Filters bar: vehicle, geofence, start date, end date (`datetime-local` → converted to ISO UTC), page size (25/50/100). Filters are stored in the URL query string (shareable, survive refresh).
- Table: time, vehicle, geofence (+ badge), event (entry/exit pill), lat/lng, "map" link.
- Pagination: "Showing 51–100 of 245" + Prev / Next, using `limit` + `offset` and `total_count`.
- New alerts invalidate the query so the first page stays fresh.

## 6. Real-time alerts

### `useAlertSocket()` hook
```ts
type ConnectionState = 'connecting' | 'open' | 'reconnecting' | 'offline';
```
- URL derived from `VITE_API_URL` (`https://` → `wss://`, `http://` → `ws://`) + `/ws/alerts`.
- Reconnect with exponential backoff + jitter: 1s, 2s, 4s … capped at 30s; resets on successful open.
- Pauses reconnection while `navigator.onLine === false`; resumes on `online`.
- Safe under React StrictMode (cleanup closes socket and cancels timers).
- Parses messages with a runtime type guard; malformed messages are ignored and logged.

### On each alert
1. Push into `AlertFeedContext` (latest 100, newest first; unread count for the bell).
2. Toast (sonner):
   - Restricted zone **entry** → red, title "🚨 KA-01-AB-1234 entered Airport (restricted)", stays 8s.
   - Others → neutral, 4s.
   - Max 3 visible; extras collapse into "+N more".
3. Invalidate queries: `['vehicleLocation', vehicleId]`, `['violations']`.
4. Update the vehicle marker position immediately from the alert's location.

### Connection indicator (top bar)
- ● green "Live" · ◌ amber pulsing "Reconnecting…" · ● gray "Offline".

## 7. Data layer

```
src/api/
  client.ts        # fetch wrapper: base URL, JSON, timeout (15s), ApiError {status, code, message}
  types.ts         # Geofence, Vehicle, VehicleLocation, AlertRule, Violation, AlertMessage
  geofences.ts     # listGeofences(category?), createGeofence(input)
  vehicles.ts      # listVehicles(), createVehicle(), updateLocation(), getVehicleLocation(id)
  alerts.ts        # listAlertRules(filters), configureAlert(), updateAlertStatus(), deleteAlertRule()
  violations.ts    # listViolations(filters)
```

Query keys: `['geofences', category]`, `['vehicles']`, `['vehicleLocation', id]`, `['alertRules', filters]`, `['violations', filters]`.
Mutations invalidate the relevant keys on success. `time_ns` is ignored by the UI (optionally shown in a dev "API timing" tooltip).

Error messages: backend `error.message` shown as-is (it is written to be user-readable); network failures → "Can't reach the server. Retrying…".

**Cold start:** the free backend host may sleep. If the first request takes > 3s, show a banner "Waking up the server — this can take up to a minute on the free tier."

## 8. States (every data view)

| State | Treatment |
|---|---|
| Loading | Skeleton rows / shimmer cards; map shows a small spinner chip |
| Empty | Icon + one line + primary action ("No geofences yet — Draw your first zone") |
| Error | Inline red panel with message + "Retry" button |
| Submitting | Button spinner + disabled; form fields stay editable after errors |
| Success | Toast + updated list (no full page reload) |

## 9. Folder structure

```
frontend/
├── index.html
├── src/
│   ├── main.tsx              # QueryClient, Router, Toaster, AlertFeedProvider
│   ├── App.tsx               # shell + routes
│   ├── api/                  # see section 7
│   ├── hooks/
│   │   ├── useAlertSocket.ts
│   │   └── useDebounce.ts
│   ├── context/AlertFeedContext.tsx
│   ├── components/
│   │   ├── layout/ (AppShell, Sidebar, BottomTabs, TopBar, ConnectionBadge)
│   │   ├── map/ (LiveMap, GeofenceLayer, VehicleMarkers, DrawControl, MapClickPicker, Legend)
│   │   ├── geofences/ (GeofenceForm, CoordinatesInput, GeofenceList)
│   │   ├── vehicles/ (VehicleForm, VehicleTable, LocationUpdateForm)
│   │   ├── alerts/ (AlertRuleForm, AlertRuleList, AlertFeed)
│   │   ├── history/ (HistoryFilters, HistoryTable, Pagination)
│   │   └── ui/ (Button, Input, Select, Badge, Modal, Sheet, Skeleton, EmptyState, ErrorState)
│   ├── pages/ (MapPage, GeofencesPage, VehiclesPage, AlertsPage, HistoryPage)
│   ├── lib/ (categories.ts colors/labels, format.ts dates & coords, validation.ts)
│   └── styles/index.css
├── vercel.json               # SPA rewrite to index.html
└── .env.example              # VITE_API_URL=http://localhost:8080
```

## 10. Accessibility

- All controls keyboard reachable; visible focus ring.
- Category never conveyed by color alone (label text + stroke pattern).
- Toasts announced via `aria-live="polite"` (assertive for restricted entries).
- Form errors linked with `aria-describedby`.
- Contrast ≥ 4.5:1 for text.

## 11. Deployment

- Vercel project rooted at `frontend/`, build `npm run build`, output `dist`.
- Env: `VITE_API_URL=https://<backend-host>`.
- `vercel.json` rewrites all routes to `/index.html` so deep links (`/history?...`) work.
- Backend `CORS_ORIGINS` must include the Vercel URL.
