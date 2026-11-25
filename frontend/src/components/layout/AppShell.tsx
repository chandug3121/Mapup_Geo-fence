import type { ReactNode } from 'react'
import { NavLink, useNavigate } from 'react-router'
import { Bell, BellRing, Car, History, Map, Shapes, Siren } from 'lucide-react'
import { useServerStatus } from '../../api/serverStatus'
import { useAlertFeed } from '../../context/AlertFeedContext'
import type { ConnectionState } from '../../hooks/useAlertSocket'
import { cx } from '../ui'

const NAV = [
  { to: '/', label: 'Live map', icon: Map, end: true },
  { to: '/geofences', label: 'Geofences', icon: Shapes },
  { to: '/vehicles', label: 'Vehicles', icon: Car },
  { to: '/alerts', label: 'Alert rules', icon: Siren },
  { to: '/history', label: 'History', icon: History },
]

export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-dvh flex-col">
      <TopBar />
      <ServerBanner />
      <div className="flex min-h-0 flex-1">
        <Sidebar />
        <main className="min-w-0 flex-1 overflow-y-auto pb-16 sm:pb-0">{children}</main>
      </div>
      <BottomTabs />
    </div>
  )
}

function TopBar() {
  const { connection, unread, markAllRead } = useAlertFeed()
  const navigate = useNavigate()
  return (
    <header className="flex h-14 shrink-0 items-center justify-between gap-3 border-b border-slate-200 bg-white px-4 dark:border-slate-800 dark:bg-slate-900">
      <div className="flex items-center gap-2">
        <img src="/favicon.svg" alt="" className="size-7" />
        <span className="font-semibold tracking-tight">Fencewatch</span>
        <span className="hidden text-sm text-slate-400 sm:inline">Geofence control center</span>
      </div>
      <div className="flex items-center gap-3">
        <ConnectionBadge state={connection} />
        <button
          type="button"
          onClick={() => {
            markAllRead()
            navigate('/?panel=feed')
          }}
          className="relative rounded-lg p-2 text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
          aria-label={unread ? `${unread} new alerts` : 'Alerts'}
        >
          {unread ? <BellRing className="size-5" /> : <Bell className="size-5" />}
          {unread > 0 && (
            <span className="absolute -top-0.5 -right-0.5 grid min-w-5 place-items-center rounded-full bg-red-600 px-1 text-[11px] font-semibold text-white">
              {unread > 99 ? '99+' : unread}
            </span>
          )}
        </button>
      </div>
    </header>
  )
}

const CONNECTION_LABEL: Record<ConnectionState, { text: string; dot: string }> = {
  open: { text: 'Live', dot: 'bg-green-500' },
  connecting: { text: 'Connecting…', dot: 'bg-amber-500 animate-pulse' },
  reconnecting: { text: 'Reconnecting…', dot: 'bg-amber-500 animate-pulse' },
  offline: { text: 'Offline', dot: 'bg-slate-400' },
}

function ConnectionBadge({ state }: { state: ConnectionState }) {
  const { text, dot } = CONNECTION_LABEL[state]
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700 dark:bg-slate-800 dark:text-slate-200"
      title="Real-time alert connection"
      role="status"
    >
      <span className={cx('size-2 rounded-full', dot)} aria-hidden />
      {text}
    </span>
  )
}

function ServerBanner() {
  const { slow, unreachable } = useServerStatus()
  if (!slow && !unreachable) return null
  return (
    <div role="status" className="bg-amber-50 px-4 py-2 text-center text-sm text-amber-900 dark:bg-amber-950 dark:text-amber-100">
      {unreachable
        ? "Can't reach the server right now. Retrying automatically…"
        : 'Waking up the server — on the free hosting tier this can take up to a minute.'}
    </div>
  )
}

function Sidebar() {
  return (
    <nav aria-label="Main" className="hidden shrink-0 flex-col gap-1 border-r border-slate-200 bg-white p-2 sm:flex lg:w-52 dark:border-slate-800 dark:bg-slate-900">
      {NAV.map(({ to, label, icon: Icon, end }) => (
        <NavLink
          key={to}
          to={to}
          end={end}
          title={label}
          className={({ isActive }) =>
            cx(
              'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
              isActive
                ? 'bg-brand-50 text-brand-700 dark:bg-slate-800 dark:text-white'
                : 'text-slate-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800',
            )
          }
        >
          <Icon className="size-5 shrink-0" aria-hidden />
          <span className="hidden lg:inline">{label}</span>
        </NavLink>
      ))}
    </nav>
  )
}

function BottomTabs() {
  return (
    <nav
      aria-label="Main"
      className="fixed inset-x-0 bottom-0 z-[500] grid grid-cols-5 border-t border-slate-200 bg-white pb-[env(safe-area-inset-bottom)] sm:hidden dark:border-slate-800 dark:bg-slate-900"
    >
      {NAV.map(({ to, label, icon: Icon, end }) => (
        <NavLink
          key={to}
          to={to}
          end={end}
          className={({ isActive }) =>
            cx('flex flex-col items-center gap-0.5 py-2 text-[11px] font-medium', isActive ? 'text-brand-600 dark:text-white' : 'text-slate-500')
          }
        >
          <Icon className="size-5" aria-hidden />
          {label.split(' ')[0]}
        </NavLink>
      ))}
    </nav>
  )
}
