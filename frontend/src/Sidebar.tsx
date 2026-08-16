import { useState, useEffect, useRef } from 'react'
import type { Session } from './App'
import type { Project } from './ProjectDetail'
import { statusLabel, t, type Locale } from './i18n'

const STATUS_ORDER = ['waiting_input', 'permission_required', 'ai_working', 'idle', 'inactive_1h', 'inactive_3h', 'inactive_12h', 'inactive_24h', 'terminated']

const STATUS_ICON_COLOR: Record<string, { icon: string; color: string }> = {
  ai_working:          { icon: '⚡', color: '#89b4fa' },
  waiting_input:       { icon: '❓', color: '#f2cdcd' },
  permission_required: { icon: '🔐', color: '#fab387' },
  idle:                { icon: '✏️', color: '#f9e2af' },
  inactive_1h:         { icon: '⏸️', color: '#585b70' },
  inactive_3h:         { icon: '⏸️', color: '#504f6a' },
  inactive_12h:        { icon: '⏸️', color: '#484764' },
  inactive_24h:        { icon: '⏸️', color: '#45475a' },
  terminated:          { icon: '⛔', color: '#45475a' },
}

const ATTENTION_STATUSES = ['waiting_input', 'permission_required']

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime()
  const sec = Math.floor(diff / 1000)
  if (sec < 60) return `${sec}s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}h ago`
  return `${Math.floor(hr / 24)}d ago`
}

type Props = {
  sessions: Session[]
  selectedId: string | null
  selectedProjectId: string | null
  onSelect: (id: string) => void
  onSelectProject: (id: string) => void
  onDelete: (id: string) => void
  onFocus: (id: string) => void
  projects: Project[]
  sessionProjectMap: Record<string, string>
  onCreateProject: () => void
  onDeleteProject: (id: string) => void
  onArchiveProject: (id: string) => void
  onAssignSession: (sessionId: string, projectId: string | null) => void
  onReorderProjects: (ids: string[]) => void
  width: number
  locale: Locale
  statusLabels?: Record<string, string>
}

export default function Sidebar({
  sessions, selectedId, selectedProjectId, onSelect, onSelectProject, onDelete, onFocus,
  projects, sessionProjectMap, onCreateProject, onDeleteProject, onArchiveProject, onAssignSession, onReorderProjects,
  width, locale, statusLabels,
}: Props) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>(() => {
    try { return JSON.parse(localStorage.getItem('sidebar-collapsed') || '{}') } catch { return {} }
  })
  const [showArchived, setShowArchived] = useState(false)
  const [dragSessionId, setDragSessionId] = useState<string | null>(null)
  const [dragProjectId, setDragProjectId] = useState<string | null>(null)
  const [editingProjectId, setEditingProjectId] = useState<string | null>(null)
  const [editName, setEditName] = useState('')
  const editRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    localStorage.setItem('sidebar-collapsed', JSON.stringify(collapsed))
  }, [collapsed])

  useEffect(() => {
    if (editingProjectId) editRef.current?.focus()
  }, [editingProjectId])

  const getStatusConfig = (status: string) => {
    const base = STATUS_ICON_COLOR[status] ?? { icon: '?', color: '#cdd6f4' }
    return { ...base, label: statusLabel(status, locale, statusLabels) }
  }

  const sessionMap = new Map(sessions.map(s => [s.session_id, s]))

  // Sessions grouped by project
  const assignedSessionIds = new Set(Object.keys(sessionProjectMap))
  const ungrouped = sessions.filter(s => !assignedSessionIds.has(s.session_id))

  const activeProjects = projects.filter(p => !p.archived)
  const archivedProjects = projects.filter(p => p.archived)

  const toggleCollapse = (id: string) => {
    setCollapsed(prev => ({ ...prev, [id]: !prev[id] }))
  }

  const getProjectBadge = (projectId: string) => {
    let count = 0
    for (const [sid, pid] of Object.entries(sessionProjectMap)) {
      if (pid !== projectId) continue
      const s = sessionMap.get(sid)
      if (s && ATTENTION_STATUSES.includes(s.status)) count++
    }
    return count
  }

  const getProjectSessionCount = (projectId: string) => {
    let count = 0
    for (const pid of Object.values(sessionProjectMap)) {
      if (pid === projectId) count++
    }
    return count
  }

  const handleDropOnProject = (projectId: string) => {
    if (dragProjectId !== null && dragProjectId !== projectId) {
      // Reorder projects
      const ids = activeProjects.map(p => p.id)
      const fromIdx = ids.indexOf(dragProjectId)
      const toIdx = ids.indexOf(projectId)
      if (fromIdx >= 0 && toIdx >= 0) {
        ids.splice(fromIdx, 1)
        ids.splice(toIdx, 0, dragProjectId)
        onReorderProjects(ids)
      }
      setDragProjectId(null)
      return
    }
    if (!dragSessionId) return
    onAssignSession(dragSessionId, projectId)
    setDragSessionId(null)
  }

  const handleDropOnUngrouped = () => {
    if (!dragSessionId) return
    onAssignSession(dragSessionId, null)
    setDragSessionId(null)
  }

  const statusGrouped = STATUS_ORDER.map(status => ({
    status,
    config: getStatusConfig(status),
    items: ungrouped.filter(s => s.status === status),
  })).filter(g => g.items.length > 0)

  const renderSession = (session: Session) => {
    const config = getStatusConfig(session.status)
    const attention = ATTENTION_STATUSES.includes(session.status)
    return (
      <div
        key={session.session_id}
        className={`sidebar-item${session.session_id === selectedId ? ' selected' : ''}${attention ? ' attention' : ''}`}
        onClick={() => onSelect(session.session_id)}
        onDoubleClick={() => onFocus(session.session_id)}
        draggable
        onDragStart={() => setDragSessionId(session.session_id)}
      >
        <div className="sidebar-item-main">
          <span
            className={`status-dot${session.status === 'ai_working' ? ' status-dot-working' : ''}`}
            style={{ background: config.color }}
          />
          <span className="sidebar-project">{session.custom_name || session.project_name}</span>
        </div>
        <div className="sidebar-item-meta">
          <span className="sidebar-time">{timeAgo(session.last_updated)}</span>
          <button
            className="delete-btn"
            onClick={e => { e.stopPropagation(); onDelete(session.session_id) }}
            title="Delete"
          >×</button>
        </div>
      </div>
    )
  }

  const renderProject = (project: Project) => {
    const badge = getProjectBadge(project.id)
    const sessionCount = getProjectSessionCount(project.id)
    const isCollapsed = collapsed[project.id]
    const projectSessions = sessions.filter(s => sessionProjectMap[s.session_id] === project.id)

    return (
      <div
        key={project.id}
        className="sidebar-group"
        onDragOver={e => e.preventDefault()}
        onDrop={() => handleDropOnProject(project.id)}
      >
        <div
          className={`sidebar-group-label group-header${project.id === selectedProjectId ? ' selected-project' : ''}`}
          draggable
          onDragStart={() => setDragProjectId(project.id)}
          onDragEnd={() => setDragProjectId(null)}
        >
          <span className="group-collapse" onClick={() => toggleCollapse(project.id)}>
            {isCollapsed ? '▸' : '▾'}
          </span>
          {editingProjectId === project.id ? (
            <input
              ref={editRef}
              className="group-name-input"
              value={editName}
              onChange={e => setEditName(e.target.value)}
              onBlur={() => {
                setEditingProjectId(null)
                if (editName.trim()) {
                  // Inline rename via API
                  fetch(`/api/projects?id=${project.id}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ ...project, name: editName.trim() }),
                  })
                }
              }}
              onKeyDown={e => {
                if (e.key === 'Enter') (e.target as HTMLInputElement).blur()
                if (e.key === 'Escape') setEditingProjectId(null)
              }}
            />
          ) : (
            <span
              className="group-name"
              onClick={() => onSelectProject(project.id)}
              onDoubleClick={() => { setEditingProjectId(project.id); setEditName(project.name) }}
            >{project.name}</span>
          )}
          {badge > 0 && <span className="group-badge">{badge}</span>}
          <span className="sidebar-group-count">{sessionCount}</span>
          <div className="project-actions">
            <button
              className="group-delete-btn"
              onClick={e => { e.stopPropagation(); onArchiveProject(project.id) }}
              title={project.archived ? t('unarchive', locale) : t('archive', locale)}
            >{project.archived ? '↩' : '📦'}</button>
            <button
              className="group-delete-btn"
              onClick={e => { e.stopPropagation(); onDeleteProject(project.id) }}
              title="Delete"
            >×</button>
          </div>
        </div>
        {!isCollapsed && projectSessions.map(renderSession)}
      </div>
    )
  }

  return (
    <aside className="sidebar" style={{ width }}>
      <div className="sidebar-header">
        <span className="sidebar-title">Sessions</span>
        <button className="group-add-btn" onClick={onCreateProject} title={t('new_project', locale)}>+</button>
      </div>
      <div className="sidebar-content">
        {activeProjects.map(renderProject)}

        {archivedProjects.length > 0 && (
          <>
            <div
              className="sidebar-archived-toggle"
              onClick={() => setShowArchived(!showArchived)}
            >
              {showArchived ? t('hide_archived', locale) : t('show_archived', locale)} ({archivedProjects.length})
            </div>
            {showArchived && archivedProjects.map(renderProject)}
          </>
        )}

        <div
          className="sidebar-ungrouped"
          onDragOver={e => e.preventDefault()}
          onDrop={handleDropOnUngrouped}
        >
          {statusGrouped.map(group => (
            <div key={group.status} className="sidebar-group">
              <div className="sidebar-group-label">
                <span>{group.config.icon}</span>
                <span>{group.config.label}</span>
                <span className="sidebar-group-count">{group.items.length}</span>
              </div>
              {group.items.map(renderSession)}
            </div>
          ))}
        </div>

        {sessions.length === 0 && (
          <div className="sidebar-empty">{t('no_sessions', locale)}</div>
        )}
      </div>
    </aside>
  )
}
