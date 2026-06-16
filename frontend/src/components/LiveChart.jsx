import {
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

export default function LiveChart({ snapshots }) {
  const latest = snapshots[snapshots.length - 1] || {}

  const data = snapshots.map((s) => ({
    t: Math.round(s.elapsedSeconds ?? 0),
    avgLatency: round(s.avgLatency),
    rps: round(s.currentRps ?? s.rps),
  }))

  return (
    <div className="flex flex-col gap-6">
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <MetricCard label="Total requests" value={fmtInt(latest.total)} />
        <MetricCard
          label="Error rate"
          value={`${round(latest.errorRate)}%`}
          danger={(latest.errorRate ?? 0) > 0}
        />
        <MetricCard label="Avg latency" value={`${round(latest.avgLatency)} ms`} />
        <MetricCard
          label="Current RPS"
          value={fmtInt(Math.round(latest.currentRps ?? 0))}
        />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <ChartCard title="Avg latency (ms)">
          <Chart data={data} dataKey="avgLatency" color="#60a5fa" />
        </ChartCard>
        <ChartCard title="Requests / sec">
          <Chart data={data} dataKey="rps" color="#34d399" />
        </ChartCard>
      </div>
    </div>
  )
}

function Chart({ data, dataKey, color }) {
  return (
    <ResponsiveContainer width="100%" height={200}>
      <LineChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: -16 }}>
        <XAxis
          dataKey="t"
          stroke="#525252"
          fontSize={11}
          tickLine={false}
          axisLine={false}
          unit="s"
        />
        <YAxis
          stroke="#525252"
          fontSize={11}
          tickLine={false}
          axisLine={false}
          width={48}
        />
        <Tooltip
          contentStyle={{
            background: '#0a0a0a',
            border: '1px solid #262626',
            borderRadius: 8,
            fontSize: 12,
          }}
          labelStyle={{ color: '#a3a3a3' }}
          labelFormatter={(t) => `${t}s`}
        />
        <Line
          type="monotone"
          dataKey={dataKey}
          stroke={color}
          strokeWidth={2}
          dot={false}
          isAnimationActive={false}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}

function ChartCard({ title, children }) {
  return (
    <div className="rounded-lg border border-neutral-800 bg-neutral-900/50 p-4">
      <h3 className="mb-3 text-xs font-medium uppercase tracking-wide text-neutral-500">
        {title}
      </h3>
      {children}
    </div>
  )
}

function MetricCard({ label, value, danger }) {
  return (
    <div className="rounded-lg border border-neutral-800 bg-neutral-900/50 p-4">
      <div className="text-xs font-medium uppercase tracking-wide text-neutral-500">
        {label}
      </div>
      <div
        className={`mt-2 text-2xl font-semibold tabular-nums ${
          danger ? 'text-red-400' : 'text-neutral-100'
        }`}
      >
        {value}
      </div>
    </div>
  )
}

function round(n) {
  if (n == null || Number.isNaN(n)) return 0
  return Math.round(n * 10) / 10
}

function fmtInt(n) {
  return (n ?? 0).toLocaleString()
}
