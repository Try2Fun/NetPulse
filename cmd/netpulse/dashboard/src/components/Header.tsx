import { Wifi, WifiOff, Loader2, Radio } from 'lucide-react'
import type { WSStatus } from '../useNetPulse'
import type { Stats } from '../types'

interface HeaderProps {
  wsStatus: WSStatus
  stats: Stats | null
  lastUpdated: Date | null
  lastCycleMs: number | null
  searchQuery: string
  onSearch: (q: string) => void
}

export function Header({ wsStatus, stats, lastUpdated, lastCycleMs, searchQuery, onSearch }: HeaderProps) {
  const uptime = stats ? Math.round((stats.up / Math.max(stats.total, 1)) * 100) : 0

  return (
    <header style={{
      position: 'sticky',
      top: 0,
      zIndex: 50,
      background: 'rgba(5, 11, 24, 0.85)',
      borderBottom: '1px solid rgba(255,255,255,0.06)',
      backdropFilter: 'blur(24px)',
      WebkitBackdropFilter: 'blur(24px)',
    }}>
      <div style={{
        maxWidth: 1400,
        margin: '0 auto',
        padding: '14px 24px',
        display: 'flex',
        alignItems: 'center',
        gap: 24,
        flexWrap: 'wrap',
      }}>

        {/* Logo */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <Radio size={22} color="var(--accent-cyan)" />
          <span style={{
            fontSize: '1.1rem',
            fontWeight: 800,
            letterSpacing: '-0.02em',
          }} className="grad-text">NetPulse</span>
          <span style={{
            fontSize: 10,
            fontWeight: 600,
            color: 'var(--accent-cyan)',
            background: 'rgba(0,255,225,0.08)',
            border: '1px solid rgba(0,255,225,0.2)',
            borderRadius: 4,
            padding: '1px 6px',
            letterSpacing: '0.1em',
          }}>LIVE</span>
        </div>

        {/* Uptime bar */}
        {stats && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flex: '1 1 auto', maxWidth: 200 }}>
            <span style={{ fontSize: 11, color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>
              Uptime {uptime}%
            </span>
            <div style={{
              flex: 1,
              height: 4,
              background: 'rgba(255,255,255,0.06)',
              borderRadius: 2,
              overflow: 'hidden',
            }}>
              <div style={{
                height: '100%',
                width: `${uptime}%`,
                background: uptime >= 80
                  ? 'linear-gradient(90deg, var(--accent-green), var(--accent-cyan))'
                  : uptime >= 50
                  ? 'linear-gradient(90deg, var(--accent-amber), var(--accent-green))'
                  : 'linear-gradient(90deg, var(--accent-red), var(--accent-amber))',
                borderRadius: 2,
                transition: 'width 0.6s cubic-bezier(0.4,0,0.2,1)',
              }} />
            </div>
          </div>
        )}

        {/* Last update */}
        {lastUpdated && (
          <span style={{
            fontSize: 11,
            color: 'var(--text-secondary)',
            fontFamily: 'var(--font-mono)',
            whiteSpace: 'nowrap',
          }}>
            {lastUpdated.toLocaleTimeString()} · {lastCycleMs}ms
          </span>
        )}

        <div style={{ flex: 1 }} />

        {/* Search */}
        <div style={{ position: 'relative' }}>
          <span style={{
            position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)',
            color: 'var(--text-dim)', pointerEvents: 'none',
            display: 'flex', alignItems: 'center',
          }}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
            </svg>
          </span>
          <input
            className="search-input"
            type="text"
            placeholder="Buscar dispositivo…"
            value={searchQuery}
            onChange={(e) => onSearch(e.target.value)}
            id="device-search"
          />
        </div>

        {/* WS status */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <div className={`ws-dot ws-${wsStatus}`} />
          {wsStatus === 'connected' && <Wifi size={14} color="var(--accent-green)" />}
          {wsStatus === 'disconnected' && <WifiOff size={14} color="var(--accent-red)" />}
          {wsStatus === 'connecting' && <Loader2 size={14} color="var(--accent-amber)" style={{ animation: 'spin 1s linear infinite' }} />}
          <span style={{
            fontSize: 11,
            color: wsStatus === 'connected' ? 'var(--accent-green)' :
                   wsStatus === 'disconnected' ? 'var(--accent-red)' : 'var(--accent-amber)',
          }}>
            {wsStatus === 'connected' ? 'En vivo' : wsStatus === 'disconnected' ? 'Desconectado' : 'Conectando…'}
          </span>
        </div>
      </div>

      <style>{`@keyframes spin { to { transform: rotate(360deg); } }`}</style>
    </header>
  )
}
