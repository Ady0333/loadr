import { useState } from 'react'

const METHODS = ['GET', 'POST', 'PUT']

const clamp = (n, min, max) => Math.min(max, Math.max(min, n))

export default function ConfigPanel({ onRun, running }) {
  const [url, setUrl] = useState('https://httpbin.org/get')
  const [method, setMethod] = useState('GET')
  const [body, setBody] = useState('')
  const [concurrency, setConcurrency] = useState(50)
  const [duration, setDuration] = useState(30)

  const hasBody = method === 'POST' || method === 'PUT'

  const submit = (e) => {
    e.preventDefault()
    onRun({
      url,
      method,
      body: hasBody ? body : '',
      headers: hasBody ? { 'Content-Type': 'application/json' } : {},
      concurrency: Number(concurrency),
      durationSeconds: Number(duration),
    })
  }

  return (
    <form
      onSubmit={submit}
      className="self-start rounded-lg border border-neutral-800 bg-neutral-900/50 p-5 flex flex-col gap-5"
    >
      <Field label="Target URL">
        <input
          type="url"
          required
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="https://api.example.com/endpoint"
          className="w-full rounded-md bg-neutral-950 border border-neutral-800 px-3 py-2 text-sm text-neutral-100 placeholder-neutral-600 focus:outline-none focus:border-neutral-600"
        />
      </Field>

      <Field label="Method">
        <div className="grid grid-cols-3 gap-2">
          {METHODS.map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMethod(m)}
              className={`rounded-md border px-3 py-2 text-sm font-medium transition-colors ${
                method === m
                  ? 'border-neutral-500 bg-neutral-800 text-neutral-100'
                  : 'border-neutral-800 bg-neutral-950 text-neutral-400 hover:border-neutral-700'
              }`}
            >
              {m}
            </button>
          ))}
        </div>
      </Field>

      {hasBody && (
        <Field label="Request body">
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={5}
            placeholder='{ "key": "value" }'
            className="w-full rounded-md bg-neutral-950 border border-neutral-800 px-3 py-2 text-sm font-mono text-neutral-100 placeholder-neutral-600 focus:outline-none focus:border-neutral-600 resize-y"
          />
        </Field>
      )}

      <Slider
        label="Concurrent users"
        value={concurrency}
        min={1}
        max={500}
        onChange={setConcurrency}
      />

      <Slider
        label="Duration"
        value={duration}
        min={5}
        max={120}
        unit="s"
        onChange={setDuration}
      />

      <button
        type="submit"
        disabled={running}
        className="mt-1 rounded-md bg-neutral-100 px-4 py-2.5 text-sm font-semibold text-neutral-900 hover:bg-white disabled:cursor-not-allowed disabled:bg-neutral-700 disabled:text-neutral-400 transition-colors"
      >
        {running ? 'Running…' : 'Run Test'}
      </button>
    </form>
  )
}

function Field({ label, children }) {
  return (
    <label className="flex flex-col gap-2">
      <span className="text-xs font-medium uppercase tracking-wide text-neutral-500">
        {label}
      </span>
      {children}
    </label>
  )
}

function Slider({ label, value, min, max, unit = '', onChange }) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')

  const startEdit = () => {
    setDraft(String(value))
    setEditing(true)
  }

  const commit = () => {
    const parsed = Number(draft)
    if (!Number.isNaN(parsed)) {
      onChange(clamp(Math.round(parsed), min, max))
    }
    setEditing(false)
  }

  return (
    <label className="flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium uppercase tracking-wide text-neutral-500">
          {label}
        </span>
        {editing ? (
          <input
            type="number"
            autoFocus
            min={min}
            max={max}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onBlur={commit}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                commit()
              } else if (e.key === 'Escape') {
                setEditing(false)
              }
            }}
            className="w-16 rounded border border-neutral-600 bg-neutral-950 px-1.5 py-0.5 text-right text-sm font-medium text-neutral-100 tabular-nums focus:outline-none focus:border-neutral-400"
          />
        ) : (
          <span
            onDoubleClick={startEdit}
            title="Double-click to edit"
            className="cursor-text select-none text-sm font-medium text-neutral-200 tabular-nums"
          >
            {value}
            {unit}
          </span>
        )}
      </div>
      <input
        type="range"
        min={min}
        max={max}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="w-full accent-neutral-200"
      />
    </label>
  )
}
