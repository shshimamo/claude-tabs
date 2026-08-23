import { useEffect, useState, useCallback, useRef } from 'react'
import Sidebar from './Sidebar'
import SessionDetail from './SessionDetail'
import ProjectDetail from './ProjectDetail'
import type { Project } from './ProjectDetail'
import WorktreeModal from './WorktreeModal'
import DeleteConfirmModal from './DeleteConfirmModal'
import ConfigModal from './ConfigModal'
import SbxRunModal from './SbxRunModal'
import CloneModal from './CloneModal'
import CreateSbxModal from './CreateSbxModal'
import DockerfileModal from './DockerfileModal'
import { notifyLabel, t, type Locale } from './i18n'

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
  memo?: string
}

const NOTIFY_STATUSES = ['idle', 'waiting_input', 'permission_required']

function extractLabels(statuses: Record<string, { label?: string }>): Record<string, string> {
  const labels: Record<string, string> = {}
  for (const [k, v] of Object.entries(statuses)) {
    if (v.label) labels[k] = v.label
  }
  return labels
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
  const [statuses, setStatuses] = useState<Record<string, { color?: string; opacity?: number; label?: string }>>({})
  const statusesRef = useRef<Record<string, { color?: string; opacity?: number; label?: string }>>({})
  const [locale, setLocale] = useState<Locale>('en')
  const localeRef = useRef<Locale>('en')
  const [focusTerminalOnSelect, setFocusTerminalOnSelect] = useState(false)
  const focusBrowserStatusesRef = useRef<string[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [sessionProjectMap, setSessionProjectMap] = useState<Record<string, string>>({})
  const [sessionOrder, setSessionOrder] = useState<Record<string, string[]>>({})
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)

  // Load projects
  const loadProjectsData = useCallback(() => {
    fetch('/api/projects').then(r => r.json()).then(data => {
      setProjects(data.projects || [])
      setSessionProjectMap(data.session_project_map || {})
      setSessionOrder(data.session_order || {})
    }).catch(() => {})
  }, [])

  // Request notification permission + load config on mount
  useEffect(() => {
    if ('Notification' in window && Notification.permission === 'default') {
      Notification.requestPermission()
    }
    fetch('/api/config').then(r => r.json()).then(cfg => {
      if (cfg.locale === 'ja') { setLocale('ja'); localeRef.current = 'ja' }
      if (cfg.statuses) { setStatuses(cfg.statuses); statusesRef.current = cfg.statuses }
      if (cfg.focus_terminal_on_select) setFocusTerminalOnSelect(true)
      if (cfg.focus_browser_on_attention?.enable) {
        focusBrowserStatusesRef.current = cfg.focus_browser_on_attention.statuses || ['waiting_input', 'permission_required']
      }
    }).catch(() => {})
    loadProjectsData()
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
          if (oldStatus && oldStatus !== s.status && NOTIFY_STATUSES.includes(s.status)) {
            const name = s.custom_name || s.project_name
            if ('Notification' in window && Notification.permission === 'granted') {
              const n = new Notification(`claude-tabs: ${name}`, {
                body: notifyLabel(s.status, localeRef.current, extractLabels(statusesRef.current)),
              })
              const sid = s.session_id
              n.onclick = () => { window.focus(); setSelectedId(sid); setSelectedProjectId(null); n.close() }
            }
            playNotificationSound()
          }
          // Auto-select session on focus_browser_on_attention status change
          if (oldStatus && oldStatus !== s.status && focusBrowserStatusesRef.current.includes(s.status)) {
            setSelectedId(s.session_id)
            setSelectedProjectId(null)
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
    if (!info.has_worktree && !info.has_sbx) {
      executeDelete(id, false, false)
      return
    }
    setDeleteConfirm({
      id,
      hasWorktree: info.has_worktree,
      hasSbx: info.has_sbx,
      worktreePath: info.worktree_path,
      sbxName: info.sbx_name,
    })
  }, [])

  const executeDelete = useCallback(async (id: string, removeWorktree: boolean, removeSbx: boolean) => {
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

  const handleCreateProject = useCallback(async () => {
    const res = await fetch('/api/projects', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'New Project' }),
    })
    if (!res.ok) return
    const p = await res.json()
    setProjects(prev => [...prev, p])
    setSelectedProjectId(p.id)
    setSelectedId(null)
  }, [])

  const handleUpdateProject = useCallback(async (project: Project) => {
    await fetch(`/api/projects?id=${project.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(project),
    })
    setProjects(prev => prev.map(p => p.id === project.id ? project : p))
  }, [])

  const handleDeleteProject = useCallback(async (id: string) => {
    if (!confirm(t('delete_project', locale))) return
    await fetch(`/api/projects?id=${id}`, { method: 'DELETE' })
    setProjects(prev => prev.filter(p => p.id !== id))
    setSessionProjectMap(prev => {
      const next = { ...prev }
      for (const [sid, pid] of Object.entries(next)) {
        if (pid === id) delete next[sid]
      }
      return next
    })
    if (selectedProjectId === id) setSelectedProjectId(null)
  }, [locale, selectedProjectId])

  const handleArchiveProject = useCallback(async (id: string) => {
    const project = projects.find(p => p.id === id)
    if (!project) return
    const updated = { ...project, archived: !project.archived }
    await fetch(`/api/projects?id=${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(updated),
    })
    setProjects(prev => prev.map(p => p.id === id ? updated : p))
  }, [projects])

  const handleAssignSession = useCallback(async (sessionId: string, projectId: string | null) => {
    await fetch(`/api/projects/session-map?session_id=${encodeURIComponent(sessionId)}&project_id=${encodeURIComponent(projectId || '')}`, { method: 'PUT' })
    setSessionProjectMap(prev => {
      const next = { ...prev }
      if (projectId) {
        next[sessionId] = projectId
      } else {
        delete next[sessionId]
      }
      return next
    })
  }, [])

  const handleReorderProjects = useCallback(async (ids: string[]) => {
    await fetch('/api/projects/reorder', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(ids),
    })
    setProjects(prev => {
      const map = new Map(prev.map(p => [p.id, p]))
      const reordered: Project[] = []
      for (const id of ids) {
        const p = map.get(id)
        if (p) reordered.push(p)
      }
      return reordered
    })
  }, [])

  const handleRenameProject = useCallback(async (id: string, name: string) => {
    const project = projects.find(p => p.id === id)
    if (!project) return
    const updated = { ...project, name }
    await fetch(`/api/projects?id=${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(updated),
    })
    setProjects(prev => prev.map(p => p.id === id ? { ...p, name } : p))
  }, [projects])

  const handleReorderSessions = useCallback(async (projectId: string, sessionIds: string[]) => {
    await fetch(`/api/projects/session-order?project_id=${encodeURIComponent(projectId)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(sessionIds),
    })
    setSessionOrder(prev => ({ ...prev, [projectId]: sessionIds }))
  }, [])

  const handleCreateProjectWithLinks = useCallback(async ({ name, githubUrl, slackUrl }: { name: string; githubUrl: string; slackUrl: string }) => {
    const linkSections = []
    if (githubUrl) linkSections.push({ label: 'GitHub', links: [{ name: 'Link', url: githubUrl }] })
    if (slackUrl) linkSections.push({ label: 'Slack', links: [{ name: 'Link', url: slackUrl }] })
    const res = await fetch('/api/projects', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, link_sections: linkSections }),
    })
    if (!res.ok) return
    const p = await res.json()
    setProjects(prev => [...prev, p])
    setSelectedProjectId(p.id)
    setSelectedId(null)
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
  const [cloneOpen, setCloneOpen] = useState(false)
  const [createSbxOpen, setCreateSbxOpen] = useState(false)
  const [dockerfileOpen, setDockerfileOpen] = useState(false)
  const [configOpen, setConfigOpen] = useState(false)

  const DEFAULT_STATUS_COLORS: Record<string, { color: string; opacity: number }> = {
    ai_working: { color: '137, 180, 250', opacity: 0.15 },
    waiting_input: { color: '242, 205, 205', opacity: 0.15 },
    permission_required: { color: '250, 179, 135', opacity: 0.15 },
  }

  const statusLabels = extractLabels(statuses)
  const hasStatusLabels = Object.keys(statusLabels).length > 0

  const selected = sessions.find(s => s.session_id === selectedId) ?? null
  const selectedProject = projects.find(p => p.id === selectedProjectId) ?? null
  const ATTENTION_STATUSES = ['waiting_input', 'permission_required']
  const hasAttention = sessions.some(s => ATTENTION_STATUSES.includes(s.status))

  return (
    <div className="app">
      <header className={`header${hasAttention ? ' header-attention' : ''}`}>
        <span className="logo">claude-tabs</span>
        <span className="session-count">{sessions.filter(s => s.status !== 'terminated' && !s.status.startsWith('inactive_')).length} active</span>
        <button className="action-btn" onClick={() => setCloneOpen(true)}>Clone</button>
        <button className="action-btn new-wt-btn" onClick={() => setCreateSbxOpen(true)}>+ Create sbx</button>
        <button className="action-btn" onClick={() => setSbxRunOpen(true)}>Attach sbx</button>
        <button className="action-btn" onClick={() => setWtModalOpen(true)}>+ Worktree</button>
        <button className="action-btn" onClick={() => setDockerfileOpen(true)}>Dockerfile</button>
        <button className="action-btn settings-btn" onClick={() => setConfigOpen(true)}>Settings</button>
      </header>
      {wtModalOpen && <WorktreeModal onClose={() => setWtModalOpen(false)} locale={locale} />}
      {cloneOpen && <CloneModal onClose={() => setCloneOpen(false)} />}
      {createSbxOpen && <CreateSbxModal onClose={() => setCreateSbxOpen(false)} />}
      {sbxRunOpen && <SbxRunModal onClose={() => setSbxRunOpen(false)} locale={locale} onCreateProject={handleCreateProjectWithLinks} />}
      {dockerfileOpen && <DockerfileModal onClose={() => setDockerfileOpen(false)} />}
      {configOpen && <ConfigModal onClose={() => setConfigOpen(false)} />}
      {deleteConfirm && <DeleteConfirmModal
        info={deleteConfirm}
        onConfirm={(removeWt, removeSbx) => executeDelete(deleteConfirm.id, removeWt, removeSbx)}
        onCancel={() => setDeleteConfirm(null)}
        locale={locale}
      />}
      <div className="body">
        <Sidebar
          sessions={sessions}
          selectedId={selectedId}
          selectedProjectId={selectedProjectId}
          onSelect={(id: string) => { setSelectedId(id); setSelectedProjectId(null); if (focusTerminalOnSelect) handleFocus(id) }}
          onSelectProject={(id: string) => { setSelectedProjectId(id); setSelectedId(null) }}
          onDelete={handleDelete}
          onFocus={handleFocus}
          projects={projects}
          sessionProjectMap={sessionProjectMap}
          onCreateProject={handleCreateProject}
          onDeleteProject={handleDeleteProject}
          onArchiveProject={handleArchiveProject}
          onAssignSession={handleAssignSession}
          onRenameProject={handleRenameProject}
          onReorderProjects={handleReorderProjects}
          onReorderSessions={handleReorderSessions}
          sessionOrder={sessionOrder}
          width={sidebarWidth}
          locale={locale}
          statusLabels={hasStatusLabels ? statusLabels : undefined}
        />
        <div className="sidebar-resize-handle" onMouseDown={handleMouseDown} />
        <main className="main" style={(() => {
          if (!selected) return undefined
          const sc = statuses[selected.status]
          const dc = DEFAULT_STATUS_COLORS[selected.status]
          const color = sc?.color || dc?.color
          const opacity = sc?.opacity ?? dc?.opacity
          if (!color || opacity === undefined) return undefined
          return { background: `rgba(${color}, ${opacity})` }
        })()}>
          {selectedProject ? (
            <ProjectDetail project={selectedProject} onUpdate={handleUpdateProject} locale={locale} />
          ) : selected ? (
            <SessionDetail session={selected} onRename={handleRename} onSetTTY={handleSetTTY} locale={locale} statusLabels={hasStatusLabels ? statusLabels : undefined} />
          ) : (
            <div className="empty-hint">
              {t('empty_hint', locale)}
            </div>
          )}
        </main>
      </div>
    </div>
  )
}
