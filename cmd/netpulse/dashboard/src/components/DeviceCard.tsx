import { motion, AnimatePresence } from 'framer-motion'
import { Gamepad2, Tv2, Smartphone, Router, Laptop, Cpu, Printer, Camera, Cloud, MonitorSmartphone, Zap } from 'lucide-react'
import type { DeviceState } from '../types'
import { Sparkline } from './Sparkline'

const ICON_MAP: Record<string, React.ElementType> = {
  gamepad: Gamepad2,
  tv: Tv2,
  phone: Smartphone,
  router: Router,
  laptop: Laptop,
  raspberry: Cpu,
  printer: Printer,
  camera: Camera,
  cloud: Cloud,
  device: MonitorSmartphone,
}

function getLatencyColor(ms: number, status: string) {
  if (status !== 'up') return 'var(--accent-red)'
  if (ms < 10) return 'var(--accent-green)'
  if (ms < 50) return 'var(--accent-cyan)'
  if (ms < 150) return 'var(--accent-amber)'
  return 'var(--accent-red)'
}

function getLatencyWidth(ms: number) {
  if (ms <= 0) return '100%'
  // 1ms → 100%, 500ms+ → 5%
  return `${Math.max(5, 100 - (ms / 500) * 95)}%`
}

interface DeviceCardProps {
  device: DeviceState
}

export function DeviceCard({ device }: DeviceCardProps) {
  const Icon = ICON_MAP[device.icon] ?? MonitorSmartphone
  const isUp = device.status === 'up'
  const isDown = device.status === 'down'
  const latencyColor = getLatencyColor(device.latency_ms, device.status)

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 24, scale: 0.95 }}
      animate={{
        opacity: 1, y: 0, scale: 1,
        boxShadow: device.flashNew
          ? ['0 0 0px rgba(0,255,225,0)', '0 0 40px rgba(0,255,225,0.4)', '0 0 0px rgba(0,255,225,0)']
          : isUp ? '0 0 20px rgba(0,255,136,0.07)' : '0 0 20px rgba(255,60,80,0.07)',
      }}
      exit={{ opacity: 0, scale: 0.9 }}
      transition={{ duration: 0.35, ease: [0.4, 0, 0.2, 1] }}
      className={`glass ${isUp ? 'glass-up' : isDown ? 'glass-dn' : 'glass-amb'}`}
      style={{ padding: 20, position: 'relative', overflow: 'hidden' }}
    >
      {/* New device ribbon */}
      {device.flashNew && (
        <motion.div
          initial={{ opacity: 0, x: 30 }}
          animate={{ opacity: 1, x: 0 }}
          style={{
            position: 'absolute', top: 12, right: -2,
            background: 'linear-gradient(135deg, var(--accent-cyan), var(--accent-blue))',
            color: '#050b18',
            fontSize: 9,
            fontWeight: 800,
            letterSpacing: '0.1em',
            padding: '3px 10px',
            borderRadius: '4px 0 0 4px',
            textTransform: 'uppercase',
            display: 'flex', alignItems: 'center', gap: 4,
          }}
        >
          <Zap size={9} /> NUEVO
        </motion.div>
      )}

      {/* Card top row */}
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 14, marginBottom: 16 }}>
        {/* Icon */}
        <div style={{
          width: 44, height: 44,
          borderRadius: 12,
          background: isUp ? 'rgba(0,255,136,0.08)' : 'rgba(255,60,80,0.08)',
          border: `1px solid ${isUp ? 'rgba(0,255,136,0.18)' : 'rgba(255,60,80,0.18)'}`,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          flexShrink: 0,
        }}>
          <Icon size={20} color={isUp ? 'var(--accent-green)' : 'var(--accent-red)'} />
        </div>

        {/* Name + address */}
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{
            fontWeight: 600,
            fontSize: 14,
            color: 'var(--text-primary)',
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            marginBottom: 3,
          }}>
            {device.name}
          </div>
          <div style={{
            fontSize: 11,
            color: 'var(--text-secondary)',
            fontFamily: 'var(--font-mono)',
          }}>
            {device.address}
          </div>
        </div>

        {/* Status badge */}
        <div className={`badge badge-${device.status}`}>
          <span className={`dot dot-${device.status}`} />
          {device.status.toUpperCase()}
        </div>
      </div>

      {/* Latency row */}
      <div style={{ marginBottom: 14 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 6 }}>
          <span style={{ fontSize: 11, color: 'var(--text-secondary)' }}>Latencia</span>
          <span style={{
            fontFamily: 'var(--font-mono)',
            fontSize: 12,
            fontWeight: 600,
            color: latencyColor,
          }}>
            {isUp ? `${device.latency_ms} ms` : '—'}
          </span>
        </div>
        <div className="latency-bar-track">
          <motion.div
            className="latency-bar-fill"
            animate={{ width: isUp ? getLatencyWidth(device.latency_ms) : '0%' }}
            transition={{ duration: 0.5, ease: 'easeOut' }}
            style={{ background: `linear-gradient(90deg, ${latencyColor}80, ${latencyColor})` }}
          />
        </div>
      </div>

      {/* Sparkline */}
      {device.history.length > 1 && (
        <div>
          <div style={{ fontSize: 10, color: 'var(--text-dim)', marginBottom: 4, letterSpacing: '0.06em', textTransform: 'uppercase' }}>
            Historial
          </div>
          <Sparkline data={device.history} color={latencyColor} />
        </div>
      )}
    </motion.div>
  )
}
