import { motion, AnimatePresence } from 'framer-motion'
import { Zap, X } from 'lucide-react'
import { Gamepad2, Tv2, Smartphone, Router, Laptop, Cpu, Printer, Camera, Cloud, MonitorSmartphone } from 'lucide-react'
import type { NewDeviceMessage } from '../types'

const ICON_MAP: Record<string, React.ElementType> = {
  gamepad: Gamepad2, tv: Tv2, phone: Smartphone, router: Router,
  laptop: Laptop, raspberry: Cpu, printer: Printer, camera: Camera,
  cloud: Cloud, device: MonitorSmartphone,
}

interface ToastProps {
  message: NewDeviceMessage | null
  onClose: () => void
}

export function Toast({ message, onClose }: ToastProps) {
  const Icon = message ? (ICON_MAP[message.device.icon] ?? MonitorSmartphone) : MonitorSmartphone

  return (
    <div className="toast-container">
      <AnimatePresence>
        {message && (
          <motion.div
            className="toast"
            initial={{ opacity: 0, x: 80, scale: 0.9 }}
            animate={{ opacity: 1, x: 0, scale: 1 }}
            exit={{ opacity: 0, x: 80, scale: 0.9 }}
            transition={{ type: 'spring', stiffness: 320, damping: 28 }}
            id="new-device-toast"
          >
            <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
              {/* Icon */}
              <div style={{
                width: 40, height: 40, borderRadius: 10, flexShrink: 0,
                background: 'rgba(0,255,225,0.1)',
                border: '1px solid rgba(0,255,225,0.25)',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}>
                <Icon size={18} color="var(--accent-cyan)" />
              </div>

              {/* Content */}
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{
                  display: 'flex', alignItems: 'center', gap: 6,
                  marginBottom: 4,
                }}>
                  <Zap size={12} color="var(--accent-cyan)" />
                  <span style={{ fontSize: 11, fontWeight: 700, color: 'var(--accent-cyan)', letterSpacing: '0.06em', textTransform: 'uppercase' }}>
                    Dispositivo nuevo
                  </span>
                </div>
                <div style={{ fontWeight: 600, fontSize: 13, color: 'var(--text-primary)', marginBottom: 2 }}>
                  {message.device.name}
                </div>
                <div style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-secondary)' }}>
                  {message.device.address} · {message.device.latency_ms}ms
                </div>
              </div>

              {/* Close */}
              <button
                onClick={onClose}
                style={{
                  background: 'none', border: 'none', cursor: 'pointer',
                  color: 'var(--text-dim)', padding: 2, flexShrink: 0,
                }}
                aria-label="Cerrar notificación"
              >
                <X size={14} />
              </button>
            </div>

            {/* Progress bar */}
            <motion.div
              style={{
                marginTop: 12,
                height: 2,
                background: 'linear-gradient(90deg, var(--accent-cyan), var(--accent-blue))',
                borderRadius: 1,
                transformOrigin: 'left',
              }}
              initial={{ scaleX: 1 }}
              animate={{ scaleX: 0 }}
              transition={{ duration: 8, ease: 'linear' }}
            />
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
