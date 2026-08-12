import { useState, useEffect, useRef } from 'react'
import type { Session } from './App'
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

type Group = { name: string; sessionIds: string[]; collapsed: boolean }

function loadGroups(): Group[] {
  try {
    return JSON.parse(localStorage.getItem('sidebar-groups') || '[]')
  } catch { return [] }
}

function saveGroups(groups: Group[]) {
  localStorage.setItem('sidebar-groups', JSON.stringify(groups))
}

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
  onSelect: (id: string) => void
  onDelete: (id: string) => void
  onFocus: (id: string) => void
  width: number
  locale: Locale
  statusLabels?: Record<string, string>
}

export default function Sidebar({ sessions, selectedId, onSelect, onDelete, onFocus, width, locale, statusLabels }: Props) {
  const [groups, setGroups] = useState<Group[]>(loadGroups)
  const [editingGroup, setEditingGroup] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const editRef = useRef<HTMLInputElement>(null)
  const [dragSessionId, setDragSessionId] = useState<string | null>(null)
  const [dragGroupIdx, setDragGroupIdx] = useState<number | null>(null)

  useEffect(() => { saveGroups(groups) }, [groups])
  useEffect(() => { if (editingGroup !== null) editRef.current?.focus() }, [editingGroup])

  // Clean up stale session IDs (skip when sessions is empty to avoid clearing on reconnect)
  useEffect(() => {
    if (sessions.length === 0) return
    const sessionIds = new Set(sessions.map(s => s.session_id))
    const cleaned = groups.map(g => ({
      ...g,
      sessionIds: g.sessionIds.filter(id => sessionIds.has(id)),
    }))
    if (JSON.stringify(cleaned) !== JSON.stringify(groups)) setGroups(cleaned)
  }, [sessions])

  const getStatusConfig = (status: string) => {
    const base = STATUS_ICON_COLOR[status] ?? { icon: '?', color: '#cdd6f4' }
    return { ...base, label: statusLabel(status, locale, statusLabels) }
  }

  const groupedSessionIds = new Set(groups.flatMap(g => g.sessionIds))
  const ungrouped = sessions.filter(s => !groupedSessionIds.has(s.session_id))

  const statusGrouped = STATUS_ORDER.map(status => ({
    status,
    config: getStatusConfig(status),
    items: ungrouped.filter(s => s.status === status),
  })).filter(g => g.items.length > 0)

  const addGroup = () => {
    const newGroup: Group = { name: 'New Group', sessionIds: [], collapsed: false }
    setGroups([...groups, newGroup])
    setEditingGroup(groups.length)
    setEditName('New Group')
  }

  const renameGroup = (idx: number) => {
    setEditingGroup(idx)
    setEditName(groups[idx].name)
  }

  const submitRename = (idx: number) => {
    setEditingGroup(null)
    const name = editName.trim()
    if (!name) return
    setGroups(groups.map((g, i) => i === idx ? { ...g, name } : g))
  }

  const deleteGroup = (idx: number) => {
    setGroups(groups.filter((_, i) => i !== idx))
  }

  const toggleCollapse = (idx: number) => {
    setGroups(groups.map((g, i) => i === idx ? { ...g, collapsed: !g.collapsed } : g))
  }

  // Drag handlers
  const handleDragStart = (sessionId: string) => {
    setDragSessionId(sessionId)
  }

  const handleDropOnGroup = (groupIdx: number) => {
    if (dragGroupIdx !== null && dragGroupIdx !== groupIdx) {
      setGroups(prev => {
        const next = [...prev]
        const [moved] = next.splice(dragGroupIdx, 1)
        next.splice(groupIdx, 0, moved)
        return next
      })
      setDragGroupIdx(null)
      return
    }
    if (!dragSessionId) return
    setGroups(prev => {
      const next = prev.map((g, i) => ({
        ...g,
        sessionIds: g.sessionIds.filter(id => id !== dragSessionId),
      }))
      next[groupIdx] = { ...next[groupIdx], sessionIds: [...next[groupIdx].sessionIds, dragSessionId] }
      return next
    })
    setDragSessionId(null)
  }

  const handleDropOnUngrouped = () => {
    if (!dragSessionId) return
    setGroups(prev => prev.map(g => ({
      ...g,
      sessionIds: g.sessionIds.filter(id => id !== dragSessionId),
    })))
    setDragSessionId(null)
  }

  const sessionMap = new Map(sessions.map(s => [s.session_id, s]))

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
        onDragStart={() => handleDragStart(session.session_id)}
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

  const getGroupBadge = (group: Group) => {
    let count = 0
    for (const id of group.sessionIds) {
      const s = sessionMap.get(id)
      if (s && ATTENTION_STATUSES.includes(s.status)) count++
    }
    return count
  }

  return (
    <aside className="sidebar" style={{ width }}>
      <div className="sidebar-header">
        <span className="sidebar-title">Sessions</span>
        <button className="group-add-btn" onClick={addGroup} title="Add group">+</button>
      </div>
      <div className="sidebar-content">
        {groups.map((group, idx) => {
          const badge = getGroupBadge(group)
          return (
            <div
              key={idx}
              className="sidebar-group"
              onDragOver={e => e.preventDefault()}
              onDrop={() => handleDropOnGroup(idx)}
            >
              <div
                className="sidebar-group-label group-header"
                draggable
                onDragStart={() => setDragGroupIdx(idx)}
                onDragEnd={() => setDragGroupIdx(null)}
              >
                <span className="group-collapse" onClick={() => toggleCollapse(idx)}>
                  {group.collapsed ? '▸' : '▾'}
                </span>
                {editingGroup === idx ? (
                  <input
                    ref={editRef}
                    className="group-name-input"
                    value={editName}
                    onChange={e => setEditName(e.target.value)}
                    onBlur={() => submitRename(idx)}
                    onKeyDown={e => { if (e.key === 'Enter') submitRename(idx); if (e.key === 'Escape') setEditingGroup(null) }}
                  />
                ) : (
                  <span className="group-name" onDoubleClick={() => renameGroup(idx)}>{group.name}</span>
                )}
                {badge > 0 && <span className="group-badge">{badge}</span>}
                <span className="sidebar-group-count">{group.sessionIds.length}</span>
                <button
                  className="group-delete-btn"
                  onClick={() => deleteGroup(idx)}
                  title="Delete group"
                >×</button>
              </div>
              {!group.collapsed && group.sessionIds.map(id => {
                const s = sessionMap.get(id)
                return s ? renderSession(s) : null
              })}
            </div>
          )
        })}

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
