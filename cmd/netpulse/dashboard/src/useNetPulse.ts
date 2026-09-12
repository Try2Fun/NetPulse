import { useEffect, useRef, useCallback, useState } from 'react'
import type { WSMessage, DeviceState, Stats, NewDeviceMessage } from './types'

export type WSStatus = 'connecting' | 'connected' | 'disconnected'

const HISTORY_LEN = 20

// Build WS URL: in dev (Vite proxy) it goes to localhost:5173/ws → proxied to :8080/ws
// In production (served by Go) it uses the same host
const WS_URL =
  import.meta.env.DEV
    ? `ws://${window.location.hostname}:5173/ws`
    : `ws://${window.location.host}/ws`

export interface UseNetPulseReturn {
  devices: DeviceState[]
  stats: Stats | null
  lastCycleMs: number | null
  lastUpdated: Date | null
  wsStatus: WSStatus
  newDeviceToast: NewDeviceMessage | null
  clearToast: () => void
}

export function useNetPulse(): UseNetPulseReturn {
  const [devices, setDevices] = useState<DeviceState[]>([])
  const [stats, setStats] = useState<Stats | null>(null)
  const [lastCycleMs, setLastCycleMs] = useState<number | null>(null)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const [wsStatus, setWsStatus] = useState<WSStatus>('connecting')
  const [newDeviceToast, setNewDeviceToast] = useState<NewDeviceMessage | null>(null)

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimer = useRef<number | undefined>(undefined)
  const historyRef = useRef<Map<string, number[]>>(new Map())

  const clearToast = useCallback(() => setNewDeviceToast(null), [setNewDeviceToast])

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    setWsStatus('connecting')
    const ws = new WebSocket(WS_URL)
    wsRef.current = ws

    ws.onopen = () => setWsStatus('connected')

    ws.onclose = () => {
      setWsStatus('disconnected')
      // Reconnect after 3s
      reconnectTimer.current = setTimeout(connect, 3000)
    }

    ws.onerror = () => ws.close()

    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data) as WSMessage

        if (msg.type === 'cycle_update') {
          setLastCycleMs(msg.cycle_ms)
          setLastUpdated(new Date(msg.timestamp))
          setStats(msg.stats)

          setDevices(
            msg.devices.map((d) => {
              const key = d.address
              const hist = historyRef.current.get(key) ?? []
              const next = [...hist, d.latency_ms].slice(-HISTORY_LEN)
              historyRef.current.set(key, next)
              return { ...d, history: next, flashNew: d.is_new }
            })
          )

          // Remove flashNew after 6s
          setTimeout(() => {
            setDevices((prev) =>
              prev.map((d) => (d.flashNew ? { ...d, flashNew: false } : d))
            )
          }, 6000)
        }

        if (msg.type === 'new_device') {
          setNewDeviceToast(msg)
          setTimeout(() => setNewDeviceToast(null), 8000)
        }
      } catch {
        // ignore malformed frames
      }
    }
  }, [])

  useEffect(() => {
    connect()
    return () => {
      clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
    }
  }, [connect])

  return { devices, stats, lastCycleMs, lastUpdated, wsStatus, newDeviceToast, clearToast }
}
