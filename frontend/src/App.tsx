import { useEffect, useState, useCallback, useRef } from 'react'
import Sidebar from './Sidebar'
import SessionDetail from './SessionDetail'
import WorktreeModal from './WorktreeModal'
import DeleteConfirmModal from './DeleteConfirmModal'
import ConfigModal from './ConfigModal'
import SbxRunModal from './SbxRunModal'

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
  tty?: string
  last_output?: string
  last_prompt?: string
}

const NOTIFY_STATUSES: Record<string, string> = {
  idle: '入力待ち',
  waiting_input: '回答待ち',
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
  const [statusColors, setStatusColors] = useState<Record<string, { color: string; opacity: number }>>({})
  const [focusTerminalOnSelect, setFocusTerminalOnSelect] = useState(false)

  // Request notification permission + load config on mount
  useEffect(() => {
    if ('Notification' in window && Notification.permission === 'default') {
      Notification.requestPermission()
    }
    fetch('/api/config').then(r => r.json()).then(cfg => {
      if (cfg.status_colors) setStatusColors(cfg.status_colors)
      if (cfg.focus_terminal_on_select) setFocusTerminalOnSelect(true)
    }).catch(() => {})
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

  const [deleteConfirm, setDeleteConfirm] = useState<{
    id: string
    hasWorktree: boolean
    hasSbx: boolean
    worktreePath: string
    sbxName: string
  } | null>(null)

  const handleDelete = useCallback(async (id: string) => {
    const res = await fetch(`/api/sessions/delete-check?id=${id}`)
    if (!res.ok) return
    const info = await res.json()
    setDeleteConfirm({
      id,
      hasWorktree: info.has_worktree,
      hasSbx: info.has_sbx,
      worktreePath: info.worktree_path,
      sbxName: info.sbx_name,
    })
  }, [])

  const executeDelete = useCallback(async (id: string, removeWorktree: boolean, removeSbx: boolean, sendExit: boolean) => {
    if (sendExit) {
      await fetch(`/api/sessions/input?id=${encodeURIComponent(id)}&text=${encodeURIComponent('/exit')}`, { method: 'POST' })
    }
    const params = new URLSearchParams({ id })
    if (removeWorktree) params.set('remove_worktree', '1')
    if (removeSbx) params.set('remove_sbx', '1')
    const res = await fetch(`/api/sessions/delete?${params}`, { method: 'DELETE' })
    if (!res.ok) return
    setSessions(prev => {
      const next = prev.filter(s => s.session_id !== id)
      if (selectedId === id) {
        setSelectedId(next[0]?.session_id ?? null)
      }
      return next
    })
    setDeleteConfirm(null)
  }, [selectedId])

  const handleRename = useCallback(async (id: string, name: string) => {
    await fetch(`/api/sessions/name?id=${encodeURIComponent(id)}&name=${encodeURIComponent(name)}`, { method: 'POST' })
  }, [])

  const handleSetTTY = useCallback(async (id: string, tty: string) => {
    await fetch(`/api/sessions/tty?id=${encodeURIComponent(id)}&tty=${encodeURIComponent(tty)}`, { method: 'POST' })
  }, [])

  const handleFocus = useCallback(async (id: string) => {
    await fetch(`/api/sessions/focus?id=${encodeURIComponent(id)}`, { method: 'POST' })
  }, [])

  const [sidebarWidth, setSidebarWidth] = useState(() => {
    const saved = localStorage.getItem('sidebar-width')
    return saved ? parseInt(saved, 10) : 260
  })
  const dragging = useRef(false)

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    dragging.current = true
    const onMove = (e: MouseEvent) => {
      if (!dragging.current) return
      const w = Math.max(150, Math.min(600, e.clientX))
      setSidebarWidth(w)
    }
    const onUp = () => {
      dragging.current = false
      document.removeEventListener('mousemove', onMove)
      document.removeEventListener('mouseup', onUp)
      setSidebarWidth(prev => { localStorage.setItem('sidebar-width', String(prev)); return prev })
    }
    document.addEventListener('mousemove', onMove)
    document.addEventListener('mouseup', onUp)
  }, [])

  const [wtModalOpen, setWtModalOpen] = useState(false)
  const [sbxRunOpen, setSbxRunOpen] = useState(false)
  const [configOpen, setConfigOpen] = useState(false)

  const DEFAULT_STATUS_COLORS: Record<string, { color: string; opacity: number }> = {
    ai_working: { color: '137, 180, 250', opacity: 0.15 },
    waiting_input: { color: '242, 205, 205', opacity: 0.15 },
    permission_required: { color: '250, 179, 135', opacity: 0.15 },
  }

  const selected = sessions.find(s => s.session_id === selectedId) ?? null
  const ATTENTION_STATUSES = ['waiting_input', 'permission_required']
  const hasAttention = sessions.some(s => ATTENTION_STATUSES.includes(s.status))

  return (
    <div className="app">
      <header className={`header${hasAttention ? ' header-attention' : ''}`}>
        <span className="logo">claude-tabs</span>
        <span className="session-count">{sessions.filter(s => s.status !== 'terminated' && !s.status.startsWith('inactive_')).length} active</span>
        <button className="action-btn new-wt-btn" onClick={() => setWtModalOpen(true)}>+ Create sbx</button>
        <button className="action-btn" onClick={() => setSbxRunOpen(true)}>Attach sbx</button>
        <button className="action-btn settings-btn" onClick={() => setConfigOpen(true)}>Settings</button>
      </header>
      {wtModalOpen && <WorktreeModal onClose={() => setWtModalOpen(false)} />}
      {sbxRunOpen && <SbxRunModal onClose={() => setSbxRunOpen(false)} />}
      {configOpen && <ConfigModal onClose={() => setConfigOpen(false)} />}
      {deleteConfirm && <DeleteConfirmModal
        info={deleteConfirm}
        onConfirm={(removeWt, removeSbx, sendExit) => executeDelete(deleteConfirm.id, removeWt, removeSbx, sendExit)}
        onCancel={() => setDeleteConfirm(null)}
      />}
      <div className="body">
        <Sidebar
          sessions={sessions}
          selectedId={selectedId}
          onSelect={(id: string) => { setSelectedId(id); if (focusTerminalOnSelect) handleFocus(id) }}
          onDelete={handleDelete}
          onFocus={handleFocus}
          width={sidebarWidth}
        />
        <div className="sidebar-resize-handle" onMouseDown={handleMouseDown} />
        <main className="main" style={selected && (statusColors[selected.status] || DEFAULT_STATUS_COLORS[selected.status])
          ? { background: `rgba(${(statusColors[selected.status] || DEFAULT_STATUS_COLORS[selected.status]).color}, ${(statusColors[selected.status] || DEFAULT_STATUS_COLORS[selected.status]).opacity})` }
          : undefined}>
          {selected ? (
            <SessionDetail session={selected} onRename={handleRename} onSetTTY={handleSetTTY} />
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
