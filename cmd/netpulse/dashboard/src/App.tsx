import { useState, useMemo } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { useNetPulse } from './useNetPulse'
import { ParticleCanvas } from './components/ParticleCanvas'
import { Header } from './components/Header'
import { StatsBar } from './components/StatsBar'
import { DeviceCard } from './components/DeviceCard'
import { Toast } from './components/Toast'

export default function App() {
  const { devices, stats, lastCycleMs, lastUpdated, wsStatus, newDeviceToast, clearToast } = useNetPulse()
  const [searchQuery, setSearchQuery] = useState('')
  const [filter, setFilter] = useState<'all' | 'up' | 'down'>('all')

  const filtered = useMemo(() => {
    const q = searchQuery.toLowerCase()
    return devices.filter((d) => {
      const matchSearch = !q || d.name.toLowerCase().includes(q) || d.address.includes(q)
      const matchFilter = filter === 'all' || d.status === filter || (filter === 'down' && d.status !== 'up')
      return matchSearch && matchFilter
    })
  }, [devices, searchQuery, filter])

  const loading = wsStatus !== 'disconnected' && devices.length === 0

  return (
    <>
      <ParticleCanvas />

      <Header
        wsStatus={wsStatus}
        stats={stats}
        lastUpdated={lastUpdated}
        lastCycleMs={lastCycleMs}
        searchQuery={searchQuery}
        onSearch={setSearchQuery}
      />

      <main style={{ maxWidth: 1400, margin: '0 auto', padding: '32px 24px 48px', flex: 1 }}>

        {/* Stats */}
        <section style={{ marginBottom: 28 }}>
          <StatsBar stats={stats} loading={loading} />
        </section>

        {/* Filter tabs */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 24 }}>
          <span style={{ fontSize: 12, color: 'var(--text-secondary)', marginRight: 4 }}>Filtrar:</span>
          {(['all', 'up', 'down'] as const).map((f) => (
            <button
              key={f}
              id={`filter-${f}`}
              onClick={() => setFilter(f)}
              style={{
                padding: '5px 14px',
                borderRadius: 8,
                border: '1px solid',
                cursor: 'pointer',
                fontSize: 12,
                fontWeight: 600,
                fontFamily: 'var(--font-sans)',
                transition: 'all 0.2s',
                letterSpacing: '0.04em',
                background: filter === f
                  ? f === 'up' ? 'rgba(0,255,136,0.12)' : f === 'down' ? 'rgba(255,60,80,0.12)' : 'rgba(0,255,225,0.1)'
                  : 'rgba(255,255,255,0.03)',
                borderColor: filter === f
                  ? f === 'up' ? 'rgba(0,255,136,0.4)' : f === 'down' ? 'rgba(255,60,80,0.4)' : 'rgba(0,255,225,0.3)'
                  : 'rgba(255,255,255,0.07)',
                color: filter === f
                  ? f === 'up' ? 'var(--accent-green)' : f === 'down' ? 'var(--accent-red)' : 'var(--accent-cyan)'
                  : 'var(--text-secondary)',
              }}
            >
              {f === 'all' ? 'Todos' : f === 'up' ? '🟢 En línea' : '🔴 Caídos'}
            </button>
          ))}
          <span style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--text-dim)' }}>
            {filtered.length} dispositivo{filtered.length !== 1 ? 's' : ''}
          </span>
        </div>

        {/* Device grid */}
        {loading ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', minHeight: 320, gap: 16 }}>
            <motion.div
              animate={{ rotate: 360 }}
              transition={{ duration: 1.5, repeat: Infinity, ease: 'linear' }}
              style={{
                width: 56, height: 56,
                borderRadius: '50%',
                border: '3px solid rgba(0,255,225,0.08)',
                borderTopColor: 'var(--accent-cyan)',
              }}
            />
            <p style={{ color: 'var(--text-secondary)', fontSize: 14 }}>
              {wsStatus === 'connecting' ? 'Conectando al servidor…' : 'Esperando datos…'}
            </p>
          </div>
        ) : filtered.length === 0 ? (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            style={{ textAlign: 'center', padding: '80px 24px', color: 'var(--text-secondary)' }}
          >
            <div style={{ fontSize: 48, marginBottom: 16 }}>🔍</div>
            <p style={{ fontSize: 15, fontWeight: 500 }}>No se encontraron dispositivos</p>
            <p style={{ fontSize: 12, color: 'var(--text-dim)', marginTop: 6 }}>
              {searchQuery ? `Sin resultados para "${searchQuery}"` : 'Sin dispositivos en este estado'}
            </p>
          </motion.div>
        ) : (
          <div className="device-grid">
            <AnimatePresence mode="popLayout">
              {filtered.map((device) => (
                <DeviceCard key={device.address} device={device} />
              ))}
            </AnimatePresence>
          </div>
        )}
      </main>

      {/* Footer */}
      <footer style={{
        textAlign: 'center',
        padding: '16px 24px',
        borderTop: '1px solid rgba(255,255,255,0.04)',
        color: 'var(--text-dim)',
        fontSize: 11,
      }}>
        NetPulse · Monitor de red en tiempo real · Hecho con ❤️ en Go + React
      </footer>

      {/* Toast */}
      <Toast message={newDeviceToast} onClose={clearToast} />
    </>
  )
}
