import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type LibraryView = 'grid' | 'list' | 'table'
export type AppTheme = 'system' | 'light' | 'dark' | 'warm' | 'custom'

export interface AppThemeColors {
  background: string
  foreground: string
  accent: string
}

export const defaultAppThemeColors: AppThemeColors = {
  background: '#fbfbfa',
  foreground: '#252523',
  accent: '#356859'
}

interface UIState {
  sidebarCollapsed: boolean
  mobileDrawerOpen: boolean
  commandOpen: boolean
  libraryView: LibraryView
  appTheme: AppTheme
  appThemeColors: AppThemeColors
  setSidebarCollapsed: (collapsed: boolean) => void
  toggleSidebar: () => void
  setMobileDrawerOpen: (open: boolean) => void
  setCommandOpen: (open: boolean) => void
  setLibraryView: (view: LibraryView) => void
  setAppTheme: (theme: AppTheme) => void
  setAppThemeColors: (colors: Partial<AppThemeColors>) => void
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,
      mobileDrawerOpen: false,
      commandOpen: false,
      libraryView: 'grid',
      appTheme: 'system',
      appThemeColors: defaultAppThemeColors,
      setSidebarCollapsed: (sidebarCollapsed) => set({ sidebarCollapsed }),
      toggleSidebar: () => set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),
      setMobileDrawerOpen: (mobileDrawerOpen) => set({ mobileDrawerOpen }),
      setCommandOpen: (commandOpen) => set({ commandOpen }),
      setLibraryView: (libraryView) => set({ libraryView }),
      setAppTheme: (appTheme) => set({ appTheme }),
      setAppThemeColors: (colors) =>
        set((state) => ({ appThemeColors: { ...state.appThemeColors, ...colors } }))
    }),
    {
      name: 'bookflow:ui:v1',
      partialize: (state) => ({
        sidebarCollapsed: state.sidebarCollapsed,
        libraryView: state.libraryView,
        appTheme: state.appTheme,
        appThemeColors: state.appThemeColors
      })
    }
  )
)
