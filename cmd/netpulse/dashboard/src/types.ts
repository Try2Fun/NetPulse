// ─── Protocol types matching server.go ─────────────────────────

export type DeviceStatus = 'up' | 'down' | 'timeout'
export type DeviceIcon =
  | 'gamepad' | 'tv' | 'phone' | 'router' | 'laptop'
  | 'raspberry' | 'printer' | 'camera' | 'cloud' | 'device'

export interface DeviceJSON {
  name: string
  address: string
  status: DeviceStatus
  latency_ms: number
  is_new: boolean
  icon: DeviceIcon
}

export interface Stats {
  total: number
  up: number
  down: number
}

export interface CycleMessage {
  type: 'cycle_update'
  timestamp: string
  cycle_ms: number
  stats: Stats
  devices: DeviceJSON[]
}

export interface NewDeviceMessage {
  type: 'new_device'
  device: DeviceJSON
}

export type WSMessage = CycleMessage | NewDeviceMessage

// ─── Extended device with sparkline history ─────────────────────
export interface DeviceState extends DeviceJSON {
  /** last N latency readings for sparkline (ms, 0 = down) */
  history: number[]
  /** true for a few seconds after appearing for the first time */
  flashNew: boolean
}
