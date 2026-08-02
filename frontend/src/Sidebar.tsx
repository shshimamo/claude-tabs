import type { Session } from './App'

const STATUS_ORDER = ['ai_working', 'waiting_input', 'permission_required', 'idle', 'inactive_1h', 'inactive_3h', 'inactive_12h', 'inactive_24h', 'terminated']

const STATUS_CONFIG: Record<string, { label: string; icon: string; color: string }> = {
  ai_working:          { label: 'AI作業中',     icon: '⚡', color: '#89b4fa' },
  waiting_input:       { label: '回答待ち',     icon: '❓', color: '#f2cdcd' },
  permission_required: { label: '許可待ち',     icon: '🔐', color: '#fab387' },
  idle:                { label: '入力待ち',     icon: '✏️', color: '#f9e2af' },
  inactive_1h:         { label: '1時間経過',    icon: '⏸️', color: '#585b70' },
  inactive_3h:         { label: '3時間経過',    icon: '⏸️', color: '#504f6a' },
  inactive_12h:        { label: '12時間経過',   icon: '⏸️', color: '#484764' },
  inactive_24h:        { label: '24時間経過',   icon: '⏸️', color: '#45475a' },
  terminated:          { label: '終了',         icon: '⛔', color: '#45475a' },
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
}

export default function Sidebar({ sessions, selectedId, onSelect, onDelete, onFocus }: Props) {
  const grouped = STATUS_ORDER.map(status => ({
    status,
    config: STATUS_CONFIG[status] ?? { label: status, icon: '?', color: '#cdd6f4' },
    items: sessions.filter(s => s.status === status),
  })).filter(g => g.items.length > 0)

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <span className="sidebar-title">Sessions</span>
      </div>
      <div className="sidebar-content">
        {grouped.map(group => (
          <div key={group.status} className="sidebar-group">
            <div className="sidebar-group-label">
              <span>{group.config.icon}</span>
              <span>{group.config.label}</span>
              <span className="sidebar-group-count">{group.items.length}</span>
            </div>
            {group.items.map(session => {
              const attention = ['waiting_input', 'permission_required'].includes(session.status)
              return (
              <div
                key={session.session_id}
                className={`sidebar-item${session.session_id === selectedId ? ' selected' : ''}${attention ? ' attention' : ''}`}
                onClick={() => onSelect(session.session_id)}
                onDoubleClick={() => onFocus(session.session_id)}
              >
                <div className="sidebar-item-main">
                  <span
                    className={`status-dot${session.status === 'ai_working' ? ' status-dot-working' : ''}`}
                    style={{ background: group.config.color }}
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
            })}
          </div>
        ))}
        {sessions.length === 0 && (
          <div className="sidebar-empty">セッションなし</div>
        )}
      </div>
    </aside>
  )
}
