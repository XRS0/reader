export interface DeviceIdentity {
  device_key: string
  device_name: string
}

export function getDeviceIdentity(): DeviceIdentity {
  const storageKey = 'bookflow:device-key:v1'
  let deviceKey = localStorage.getItem(storageKey)
  if (!deviceKey) {
    deviceKey = crypto.randomUUID()
    localStorage.setItem(storageKey, deviceKey)
  }
  const nav = navigator as Navigator & { userAgentData?: { platform?: string; mobile?: boolean } }
  const platform = nav.userAgentData?.platform || navigator.platform || 'Web'
  const mobile = nav.userAgentData?.mobile ? 'Mobile' : 'Browser'
  return { device_key: deviceKey, device_name: `${platform} · ${mobile}` }
}
