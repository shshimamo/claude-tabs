import { useState, useEffect, useRef } from 'react'
import type { Session } from './App'

const STATUS_CONFIG: Record<string, { label: string; icon: string; className: string }> = {
  ai_working:          { label: 'AI作業中',  icon: '🔵', className: 'status-working' },
  waiting_input:       { label: '回答待ち',  icon: '❓', className: 'status-waiting' },
  permission_required: { label: '許可待ち',  icon: '🔐', className: 'status-permission' },
  idle:                { label: '入力待ち',  icon: '💤', className: 'status-idle' },
  terminated:          { label: '終了',      icon: '⛔', className: 'status-terminated' },
}

type HistoryMessage = {
  role: string
  content: string
}

function formatTime(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

type Props = {
  session: Session
  onRename: (id: string, name: string) => void
  onSetTTY: (id: string, tty: string) => void
}

export default function SessionDetail({ session, onRename, onSetTTY }: Props) {
  const config = STATUS_CONFIG[session.status] ?? { label: session.status, icon: '?', className: '' }
  const [editing, setEditing] = useState(false)
  const [editName, setEditName] = useState('')
  const [editingTTY, setEditingTTY] = useState(false)
  const [editTTY, setEditTTY] = useState('')
  const [history, setHistory] = useState<HistoryMessage[]>([])
  const [historyOpen, setHistoryOpen] = useState(false)
  const [focusing, setFocusing] = useState(false)
  const [sendText, setSendText] = useState('')
  const [sending, setSending] = useState(false)
  const [keysSent, setKeysSent] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const ttyInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => { setKeysSent(false) }, [session.status, session.last_output])

  useEffect(() => {
    if (historyOpen) {
      fetch(`/api/sessions/history?id=${session.session_id}`)
        .then(r => r.json())
        .then(setHistory)
        .catch(() => setHistory([]))
    }
  }, [session.session_id, historyOpen])

  const startEdit = () => {
    setEditName(session.custom_name || session.project_name)
    setEditing(true)
    setTimeout(() => inputRef.current?.focus(), 0)
  }

  const submitEdit = () => {
    setEditing(false)
    const name = editName.trim()
    if (name && name !== session.project_name) {
      onRename(session.session_id, name)
    } else if (!name || name === session.project_name) {
      onRename(session.session_id, '')
    }
  }

  const startEditTTY = () => {
    setEditTTY(session.tty || '')
    setEditingTTY(true)
    setTimeout(() => ttyInputRef.current?.focus(), 0)
  }

  const submitTTY = () => {
    setEditingTTY(false)
    onSetTTY(session.session_id, editTTY.trim())
  }

  const handleSendKeys = async (action: string) => {
    setSending(true)
    try {
      await fetch(`/api/sessions/keys?id=${session.session_id}&action=${action}`, { method: 'POST' })
      setKeysSent(true)
    } catch { /* ignore */ }
    setSending(false)
  }

  const handleSendInput = async (text: string) => {
    setSending(true)
    try {
      await fetch(`/api/sessions/input?id=${session.session_id}&text=${encodeURIComponent(text)}`, { method: 'POST' })
    } catch { /* ignore */ }
    setSending(false)
    setSendText('')
  }

  const handleFocus = async () => {
    setFocusing(true)
    try {
      await fetch(`/api/sessions/focus?id=${session.session_id}`, { method: 'POST' })
    } catch { /* ignore */ }
    setFocusing(false)
  }

  const displayName = session.custom_name || session.project_name

  return (
    <div className="detail">
      <div className="detail-header">
        {editing ? (
          <input
            ref={inputRef}
            className="detail-name-input"
            value={editName}
            onChange={e => setEditName(e.target.value)}
            onBlur={submitEdit}
            onKeyDown={e => { if (e.key === 'Enter') submitEdit(); if (e.key === 'Escape') setEditing(false) }}
          />
        ) : (
          <h2 className="detail-project" onClick={startEdit} title="Click to rename">
            {displayName}
            <span className="edit-icon">✎</span>
          </h2>
        )}
        <span className={`detail-status ${config.className}`}>
          {config.icon} {config.label}
        </span>
      </div>

      <div className="detail-actions">
        {session.status !== 'terminated' && (session.pid > 0 || session.tty) && (
          <button className="action-btn focus-btn" onClick={handleFocus} disabled={focusing}>
            {focusing ? 'Focusing...' : '🖥 Focus Terminal'}
          </button>
        )}
      </div>

      {session.status === 'permission_required' && (session.pid > 0 || session.tty) && (
        <div className="detail-input-section">
          <div className="detail-input-label">許可選択</div>
          {keysSent ? (
            <div className="keys-sent-msg">送信済み ✓</div>
          ) : (
            <div className="detail-input-row">
              <button className="action-btn allow-btn" onClick={() => handleSendKeys('allow')} disabled={sending}>✅ Allow</button>
              <button className="action-btn allow-always-btn" onClick={() => handleSendKeys('allow_always')} disabled={sending}>🔓 Allow Always</button>
              <button className="action-btn deny-btn" onClick={() => handleSendKeys('deny')} disabled={sending}>❌ Deny</button>
            </div>
          )}
        </div>
      )}

      {['idle', 'waiting_input', 'permission_required'].includes(session.status) && (session.pid > 0 || session.tty) && (
        <div className="detail-input-section">
          <div className="detail-input-label">定型文</div>
          <div className="detail-input-row">
            <button className="action-btn" onClick={() => handleSendInput('yes')} disabled={sending}>Yes</button>
            <button className="action-btn" onClick={() => handleSendInput('commit して')} disabled={sending}>Commit</button>
            <button className="action-btn" onClick={() => handleSendInput('commit して push して')} disabled={sending}>Commit & Push</button>
          </div>
          <div className="detail-input-label" style={{ marginTop: 12 }}>自由入力</div>
          <div className="detail-input-row">
            <textarea
              className="detail-send-input detail-send-textarea"
              value={sendText}
              placeholder="入力を送信..."
              onChange={e => setSendText(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey && sendText.trim()) { e.preventDefault(); handleSendInput(sendText.trim()) } }}
              disabled={sending}
              rows={2}
            />
            <button className="action-btn" onClick={() => handleSendInput(sendText.trim())} disabled={sending || !sendText.trim()}>
              送信
            </button>
          </div>
        </div>
      )}

      {session.question && (
        <div className="detail-question">
          <div className="detail-question-label">Question</div>
          <div className="detail-question-text">{session.question}</div>
        </div>
      )}

      {session.last_prompt && (
        <div className="detail-question">
          <div className="detail-question-label" style={{ color: '#89b4fa' }}>User Input</div>
          <div className="detail-question-text">{session.last_prompt}</div>
        </div>
      )}

      {session.last_output && (
        <div className="detail-question">
          <div className="detail-question-label" style={{ color: session.status === 'permission_required' ? '#fab387' : '#a6e3a1' }}>
            {session.status === 'permission_required' ? 'Permission Request' : 'Last Output'}
          </div>
          <div className="detail-question-text">{session.last_output}</div>
        </div>
      )}

      <div className="detail-grid">
        <div className="detail-field">
          <span className="detail-label">CWD</span>
          <span className="detail-value detail-mono">{session.cwd}</span>
        </div>
        <div className="detail-field">
          <span className="detail-label">PID</span>
          <span className="detail-value detail-mono">{session.pid || '-'}</span>
        </div>
        <div className="detail-field">
          <span className="detail-label">TTY</span>
          {editingTTY ? (
            <input
              ref={ttyInputRef}
              className="detail-name-input"
              style={{ fontSize: 14 }}
              value={editTTY}
              placeholder="/dev/ttys001 (sbx等リモート環境のFocus Terminal用)"
              onChange={e => setEditTTY(e.target.value)}
              onBlur={submitTTY}
              onKeyDown={e => { if (e.key === 'Enter') submitTTY(); if (e.key === 'Escape') setEditingTTY(false) }}
            />
          ) : (
            <span className="detail-value detail-mono" onClick={startEditTTY} style={{ cursor: 'pointer' }}>
              {session.tty || <span style={{ color: '#6c7086' }}>Click to set TTY</span>}
              <span className="edit-icon" style={{ opacity: 0.5, marginLeft: 6 }}>✎</span>
            </span>
          )}
        </div>
        <div className="detail-field">
          <span className="detail-label">Session ID</span>
          <span className="detail-value detail-mono">{session.session_id}</span>
        </div>
        <div className="detail-field">
          <span className="detail-label">Last Event</span>
          <span className="detail-value">{session.last_event}</span>
        </div>
        <div className="detail-field">
          <span className="detail-label">Last Updated</span>
          <span className="detail-value">{formatTime(session.last_updated)}</span>
        </div>
      </div>

      <div className="history-section">
        <button className="history-toggle" onClick={() => setHistoryOpen(v => !v)}>
          {historyOpen ? '▾' : '▸'} Conversation History
          {history.length > 0 && <span className="history-count">{history.length}</span>}
        </button>
        {historyOpen && (
          <div className="history-list">
            {history.length === 0 && (
              <div className="history-empty">会話履歴が見つからない</div>
            )}
            {history.map((msg, i) => (
              <div key={i} className={`history-msg history-${msg.role}`}>
                <div className="history-role">{msg.role === 'user' ? 'You' : 'Claude'}</div>
                <div className="history-content">{msg.content}</div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
