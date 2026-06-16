import { useCallback, useState } from 'react'
import { useWebSocket } from './hooks/useWebSocket'
import ConfigPanel from './components/ConfigPanel'
import LiveChart from './components/LiveChart'
import AIReport from './components/AIReport'

const WS_URL = 'ws://localhost:8080/ws'

export default function App() {
  const [testRunning, setTestRunning] = useState(false)
  const [snapshots, setSnapshots] = useState([])
  const [aiReport, setAiReport] = useState('')
  const [aiLoading, setAiLoading] = useState(false)

  // Ask the backend to analyze a completed run.
  const fetchReport = useCallback(async (runSnapshots) => {
    setAiLoading(true)
    setAiReport('')
    try {
      const res = await fetch('/api/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(runSnapshots),
      })
      if (!res.ok) throw new Error(`analyze failed (${res.status})`)
      setAiReport(await res.text())
    } catch (err) {
      setAiReport(`Could not load AI analysis: ${err.message}`)
    } finally {
      setAiLoading(false)
    }
  }, [])

  // Each WebSocket message is a MetricsSnapshot. The final one ends the run.
  const handleSnapshot = useCallback(
    (snap) => {
      setSnapshots((prev) => {
        const next = [...prev, snap]
        if (snap.final) {
          setTestRunning(false)
          fetchReport(next)
        }
        return next
      })
    },
    [fetchReport],
  )

  const { connected } = useWebSocket(WS_URL, handleSnapshot)

  const handleRun = useCallback(async (config) => {
    setSnapshots([])
    setAiReport('')
    setTestRunning(true)
    try {
      const res = await fetch('/api/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      })
      if (!res.ok) {
        const text = await res.text()
        throw new Error(text || `run failed (${res.status})`)
      }
    } catch (err) {
      setTestRunning(false)
      setAiReport(`Failed to start test: ${err.message}`)
    }
  }, [])

  return (
    <div className="min-h-full">
      <header className="border-b border-neutral-800 px-6 py-4 flex items-center justify-between">
        <div className="flex items-baseline gap-3">
          <h1 className="text-xl font-semibold tracking-tight text-neutral-100">
            Loadr
          </h1>
          <span className="text-sm text-neutral-500">HTTP load testing</span>
        </div>
        <div className="flex items-center gap-2 text-sm text-neutral-400">
          <span
            className={`h-2 w-2 rounded-full ${
              connected ? 'bg-emerald-500' : 'bg-neutral-600'
            }`}
          />
          {connected ? 'Connected' : 'Disconnected'}
        </div>
      </header>

      <main className="grid grid-cols-1 lg:grid-cols-[320px_1fr] gap-6 p-6">
        <ConfigPanel onRun={handleRun} running={testRunning} />

        <div className="flex flex-col gap-6 min-w-0">
          <LiveChart snapshots={snapshots} />
          <AIReport report={aiReport} loading={aiLoading || testRunning} />
        </div>
      </main>
    </div>
  )
}
