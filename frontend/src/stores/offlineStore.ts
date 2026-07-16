import { create } from 'zustand'

interface OfflineState {
  online: boolean
  pending: number
  syncing: boolean
  setOnline: (online: boolean) => void
  setPending: (pending: number) => void
  setSyncing: (syncing: boolean) => void
}

export const useOfflineStore = create<OfflineState>((set) => ({
  online: typeof navigator === 'undefined' ? true : navigator.onLine,
  pending: 0,
  syncing: false,
  setOnline: (online) => set({ online }),
  setPending: (pending) => set({ pending }),
  setSyncing: (syncing) => set({ syncing })
}))
