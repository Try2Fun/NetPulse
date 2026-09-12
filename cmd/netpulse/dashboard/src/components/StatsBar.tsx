import { motion, AnimatePresence } from 'framer-motion'
import { TrendingUp, Server, AlertCircle } from 'lucide-react'
import type { Stats } from '../types'

interface StatsBarProps {
  stats: Stats | null
  loading: boolean
}

interface StatItemProps {
  label: string
  value: number | string
  color: string
  icon: React.ElementType
  sub?: string
}

function StatItem({ label, value, color, icon: Icon, sub }: StatItemProps) {
  return (
    <div className="glass" style={{
      padding: '18px 24px',
      display: 'flex',
      alignItems: 'center',
      gap: 16,
      flex: '1 1 160px',
      minWidth: 0,
    }}>
      <div style={{
        width: 48, height: 48,
        borderRadius: 14,
        background: `${color}14`,
        border: `1px solid ${color}30`,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        flexShrink: 0,
      }}>
        <Icon size={22} color={color} />
      </div>
      <div>
        <div style={{ fontSize: 11, color: 'var(--text-secondary)', marginBottom: 4, letterSpacing: '0.05em', textTransform: 'uppercase' }}>
          {label}
        </div>
        <motion.div
          className="stat-value"
          key={String(value)}
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          style={{ color }}
        >
          {value}
        </motion.div>
        {sub && (
          <div style={{ fontSize: 10, color: 'var(--text-dim)', marginTop: 2 }}>{sub}</div>
        )}
      </div>
    </div>
  )
}

export function StatsBar({ stats, loading }: StatsBarProps) {
  if (loading || !stats) {
    return (
      <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
        {[1, 2, 3].map((i) => (
          <div key={i} className="glass skeleton" style={{ flex: '1 1 160px', height: 84 }} />
        ))}
      </div>
    )
  }

  const uptime = Math.round((stats.up / Math.max(stats.total, 1)) * 100)

  return (
    <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
      <StatItem
        label="Total dispositivos"
        value={stats.total}
        color="var(--accent-cyan)"
        icon={Server}
      />
      <StatItem
        label="En línea"
        value={stats.up}
        color="var(--accent-green)"
        icon={TrendingUp}
        sub={`${uptime}% uptime`}
      />
      <StatItem
        label="Caídos / Timeout"
        value={stats.down}
        color={stats.down > 0 ? 'var(--accent-red)' : 'var(--text-secondary)'}
        icon={AlertCircle}
        sub={stats.down === 0 ? 'Todo OK 🎉' : undefined}
      />
    </div>
  )
}
