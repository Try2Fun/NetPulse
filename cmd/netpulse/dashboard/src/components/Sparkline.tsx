interface SparklineProps {
  data: number[]
  color: string
  width?: number
  height?: number
}

export function Sparkline({ data, color, width = 240, height = 36 }: SparklineProps) {
  if (data.length < 2) return null

  const positiveData = data.map((v) => Math.max(0, v))
  const max = Math.max(...positiveData, 1)
  const min = 0

  const pts = positiveData.map((v, i) => {
    const x = (i / (data.length - 1)) * width
    const y = height - ((v - min) / (max - min)) * (height - 4) - 2
    return [x, y] as [number, number]
  })

  // Smooth SVG path using cubic bezier
  const d = pts.reduce((acc, [x, y], i) => {
    if (i === 0) return `M ${x},${y}`
    const [px, py] = pts[i - 1]
    const cpx = (px + x) / 2
    return `${acc} C ${cpx},${py} ${cpx},${y} ${x},${y}`
  }, '')

  // Fill path (close to bottom)
  const fillD = `${d} L ${pts[pts.length - 1][0]},${height} L ${pts[0][0]},${height} Z`

  return (
    <svg
      className="sparkline"
      viewBox={`0 0 ${width} ${height}`}
      width="100%"
      height={height}
      preserveAspectRatio="none"
    >
      <defs>
        <linearGradient id={`sg-${color.replace(/[^a-z0-9]/gi, '')}`} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.3" />
          <stop offset="100%" stopColor={color} stopOpacity="0.01" />
        </linearGradient>
      </defs>
      {/* Fill area */}
      <path
        d={fillD}
        fill={`url(#sg-${color.replace(/[^a-z0-9]/gi, '')})`}
      />
      {/* Line */}
      <path
        d={d}
        fill="none"
        stroke={color}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        opacity={0.8}
      />
      {/* Last point dot */}
      {pts.length > 0 && (
        <circle
          cx={pts[pts.length - 1][0]}
          cy={pts[pts.length - 1][1]}
          r={2.5}
          fill={color}
          opacity={0.9}
        />
      )}
    </svg>
  )
}
