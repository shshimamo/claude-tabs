import { useEffect, useState, useCallback, useRef } from 'react'
import Sidebar from './Sidebar'
import SessionDetail from './SessionDetail'

export type Session = {
  session_id: string
  pid: number
  cwd: string
  status: string
  last_event: string
  last_updated: string
  question?: string
  project_name: string
  custom_name?: string
}

const NOTIFY_STATUSES: Record<string, string> = {
  idle: '入力待ち',
  waiting_input: '質問待ち',
  permission_required: '許可待ち',
}

let audioCtx: AudioContext | null = null

function ensureAudioContext() {
  if (!audioCtx) audioCtx = new AudioContext()
  if (audioCtx.state === 'suspended') audioCtx.resume()
  return audioCtx
}

// Initialize AudioContext on first user interaction
if (typeof window !== 'undefined') {
  const initAudio = () => {
    ensureAudioContext()
    window.removeEventListener('click', initAudio)
    window.removeEventListener('keydown', initAudio)
  }
  window.addEventListener('click', initAudio)
  window.addEventListener('keydown', initAudio)
}

function playNotificationSound() {
  const ctx = ensureAudioContext()
  const osc = ctx.createOscillator()
  const gain = ctx.createGain()
  osc.connect(gain)
  gain.connect(ctx.destination)
  osc.frequency.value = 800
  gain.gain.setValueAtTime(0.3, ctx.currentTime)
  gain.gain.exponentialRampToValueAtTime(0.01, ctx.currentTime + 0.3)
  osc.start(ctx.currentTime)
  osc.stop(ctx.currentTime + 0.3)
}

export default function App() {
  const [sessions, setSessions] = useState<Session[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const prevSessionsRef = useRef<Map<string, string>>(new Map())

  // Request notification permission on mount
  useEffect(() => {
    if ('Notification' in window && Notification.permission === 'default') {
      Notification.requestPermission()
    }
  }, [])

  // WebSocket connection with auto-reconnect
  useEffect(() => {
    let alive = true

    function connect() {
      if (!alive) return
      const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
      const ws = new WebSocket(`${protocol}//${location.host}/api/ws`)
      wsRef.current = ws

      ws.onmessage = (e) => {
        const data: Session[] = JSON.parse(e.data)

        // Detect status changes and notify
        const prev = prevSessionsRef.current
        for (const s of data) {
          const oldStatus = prev.get(s.session_id)
          if (oldStatus && oldStatus !== s.status && s.status in NOTIFY_STATUSES) {
            const name = s.custom_name || s.project_name
            if ('Notification' in window && Notification.permission === 'granted') {
              const n = new Notification(`claude-tabs: ${name}`, {
                body: NOTIFY_STATUSES[s.status],
              })
              const sid = s.session_id
              n.onclick = () => { window.focus(); setSelectedId(sid); n.close() }
            }
            playNotificationSound()
          }
        }
        // Update prev state
        const newMap = new Map<string, string>()
        for (const s of data) newMap.set(s.session_id, s.status)
        prevSessionsRef.current = newMap

        setSessions(data)
        setSelectedId(prev => {
          if (prev && data.some(s => s.session_id === prev)) return prev
          return data.length > 0 ? data[0].session_id : null
        })
      }

      ws.onclose = () => {
        wsRef.current = null
        if (alive) setTimeout(connect, 3000)
      }
    }

    connect()
    return () => { alive = false; wsRef.current?.close() }
  }, [])

  const handleDelete = useCallback(async (id: string) => {
    const res = await fetch(`/api/sessions/delete?id=${id}`, { method: 'DELETE' })
    if (!res.ok) return
    setSessions(prev => {
      const next = prev.filter(s => s.session_id !== id)
      if (selectedId === id) {
        setSelectedId(next[0]?.session_id ?? null)
      }
      return next
    })
  }, [selectedId])

  const handleRename = useCallback(async (id: string, name: string) => {
    await fetch(`/api/sessions/name?id=${encodeURIComponent(id)}&name=${encodeURIComponent(name)}`, { method: 'POST' })
  }, [])

  const selected = sessions.find(s => s.session_id === selectedId) ?? null

  return (
    <div className="app">
      <header className="header">
        <span className="logo">claude-tabs</span>
        <span className="session-count">{sessions.filter(s => s.status !== 'terminated').length} active</span>
      </header>
      <div className="body">
        <Sidebar
          sessions={sessions}
          selectedId={selectedId}
          onSelect={setSelectedId}
          onDelete={handleDelete}
        />
        <main className="main">
          {selected ? (
            <SessionDetail session={selected} onRename={handleRename} />
          ) : (
            <div className="empty-hint">
              セッションなし。Claude Code hooks を設定してセッションを開始してください。
            </div>
          )}
        </main>
      </div>
    </div>
  )
}
