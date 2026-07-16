const demo = import.meta.env.VITE_DEMO_MODE === 'true'

export const apiConfig = {
  baseUrl: (import.meta.env.VITE_API_BASE_URL || '/api/v1').replace(/\/$/, ''),
  demo,
  heartbeatMs: Math.max(5_000, Number(import.meta.env.VITE_READING_HEARTBEAT_MS) || 15_000)
} as const
