import { Route, Routes } from 'react-router'
import { AppShell } from './components/layout/AppShell'
import { EmptyState } from './components/ui'
import { AlertsPage } from './pages/AlertsPage'
import { GeofencesPage } from './pages/GeofencesPage'
import { HistoryPage } from './pages/HistoryPage'
import { MapPage } from './pages/MapPage'
import { VehiclesPage } from './pages/VehiclesPage'

export function App() {
  return (
    <AppShell>
      <Routes>
        <Route index element={<MapPage />} />
        <Route path="geofences" element={<GeofencesPage />} />
        <Route path="vehicles" element={<VehiclesPage />} />
        <Route path="alerts" element={<AlertsPage />} />
        <Route path="history" element={<HistoryPage />} />
        <Route path="*" element={<EmptyState title="Page not found" body="Use the navigation to get back on track." />} />
      </Routes>
    </AppShell>
  )
}
