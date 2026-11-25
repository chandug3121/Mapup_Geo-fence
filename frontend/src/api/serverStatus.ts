import { useSyncExternalStore } from 'react'

/**
 * Tracks whether API requests are unusually slow (the free backend tier is
 * waking up) or failing, so the app can show one banner instead of every
 * screen handling it separately.
 */
export interface ServerState {
  slow: boolean
  unreachable: boolean
}

let state: ServerState = { slow: false, unreachable: false }
const listeners = new Set<() => void>()
const slowTickets = new Set<number>()
const timers = new Map<number, ReturnType<typeof setTimeout>>()
let nextTicket = 1

function set(next: Partial<ServerState>) {
  const merged = { ...state, ...next }
  if (merged.slow === state.slow && merged.unreachable === state.unreachable) return
  state = merged
  listeners.forEach((l) => l())
}

export const serverStatus = {
  begin(slowAfterMs: number): number {
    const ticket = nextTicket++
    timers.set(
      ticket,
      setTimeout(() => {
        slowTickets.add(ticket)
        set({ slow: true })
      }, slowAfterMs),
    )
    return ticket
  },
  end(ticket: number) {
    clearTimeout(timers.get(ticket))
    timers.delete(ticket)
    slowTickets.delete(ticket)
    if (slowTickets.size === 0) set({ slow: false })
  },
  reachable() {
    set({ unreachable: false })
  },
  unreachable() {
    set({ unreachable: true })
  },
  subscribe(listener: () => void) {
    listeners.add(listener)
    return () => listeners.delete(listener)
  },
  get: () => state,
}

export function useServerStatus(): ServerState {
  return useSyncExternalStore(serverStatus.subscribe, serverStatus.get, serverStatus.get)
}
