import { useState, useEffect, useRef } from 'react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import mermaid from 'mermaid'
import { ShikiHighlighter } from 'react-shiki'
import type { Session } from './App'
import { statusLabel, t, type Locale } from './i18n'

mermaid.initialize({ startOnLoad: false, theme: 'dark' })

function MermaidBlock({ code, light }: { code: string; light?: boolean }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!ref.current) return
    mermaid.initialize({ startOnLoad: false, theme: light ? 'default' : 'dark' })
    const id = `mermaid-${Date.now()}-${Math.random().toString(36).slice(2)}`
    mermaid.render(id, code).then(({ svg }) => {
      if (ref.current) ref.current.innerHTML = svg
    }).catch(() => {
      if (ref.current) ref.current.textContent = code
    })
  }, [code, light])
  return <div ref={ref} />
}

const STATUS_ICON_CLASS: Record<string, { icon: string; className: string }> = {
  ai_working:          { icon: '⚡', className: 'status-working' },
  waiting_input:       { icon: '❓', className: 'status-waiting' },
  permission_required: { icon: '🔐', className: 'status-permission' },
  idle:                { icon: '✏️', className: 'status-idle' },
  terminated:          { icon: '⛔', className: 'status-terminated' },
}

type HistoryMessage = {
  role: string
  content: string
}

function formatTime(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function unescapeUnicode(s: string): string {
  return s
    .replace(/\\u([0-9a-fA-F]{4})/g, (_, hex) => String.fromCharCode(parseInt(hex, 16)))
    .replace(/\\n/g, '\n')
    .replace(/\\t/g, '\t')
}

type Props = {
  session: Session
  onRename: (id: string, name: string) => void
  onSetTTY: (id: string, tty: string) => void
  locale: Locale
  statusLabels?: Record<string, string>
}

export default function SessionDetail({ session, onRename, onSetTTY, locale, statusLabels }: Props) {
  const base = STATUS_ICON_CLASS[session.status] ?? { icon: '?', className: '' }
  const config = { ...base, label: statusLabel(session.status, locale, statusLabels) }
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
  const [presets, setPresets] = useState<{ label: string; text: string }[]>([])
  const [cwdBases, setCwdBases] = useState<string[]>([])
  const [memo, setMemo] = useState('')
  const [memoOpen, setMemoOpen] = useState(false)
  const [screenContent, setScreenContent] = useState('')
  const [screenLoading, setScreenLoading] = useState(false)
  const [previewInterval, setPreviewInterval] = useState(10000)
  const [conversationHeight, setConversationHeight] = useState('70vh')
  const [contentHeight, setContentHeight] = useState('200px')
  const [outputMode, setOutputMode] = useState<'text' | 'md'>(() => {
    return (localStorage.getItem('output-mode') as 'text' | 'md') || 'text'
  })
  const [outputLight, setOutputLight] = useState(() => {
    return localStorage.getItem('output-light') === '1'
  })
  const [savedOutputs, setSavedOutputs] = useState<{ output: string; input?: string; savedAt: string; favorite?: boolean }[]>([])
  const [savedOpen, setSavedOpen] = useState(false)
  const [favFilter, setFavFilter] = useState(false)
  const [infoOpen, setInfoOpen] = useState(false)
  const [expandedSaved, setExpandedSaved] = useState<Set<number>>(new Set())
  const inputRef = useRef<HTMLInputElement>(null)
  const ttyInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => { setKeysSent(false) }, [session.status, session.last_output])

  // Sync memo from session
  useEffect(() => {
    setMemo(session.memo || '')
  }, [session.session_id])

  const fetchConversations = () => {
    fetch(`/api/conversations?session_id=${session.session_id}`)
      .then(r => r.json())
      .then((data: { output: string; input?: string; saved_at: string; favorite?: boolean }[]) => {
        setSavedOutputs(data.map(d => ({ output: d.output, input: d.input, savedAt: d.saved_at, favorite: d.favorite })))
      })
      .catch(() => setSavedOutputs([]))
  }

  // Load conversations from API on session change
  useEffect(() => {
    fetchConversations()
    setSavedOpen(false)
    setExpandedSaved(new Set([0]))
  }, [session.session_id])

  // Reload conversations when last_output changes (backend auto-saves)
  useEffect(() => {
    if (!session.last_output) return
    fetchConversations()
  }, [session.last_output])

  const deleteSavedOutput = (index: number) => {
    fetch(`/api/conversations?session_id=${session.session_id}&index=${index}`, { method: 'DELETE' })
      .then(() => setSavedOutputs(prev => prev.filter((_, i) => i !== index)))
      .catch(() => {})
  }

  const deleteNonFavorites = () => {
    fetch(`/api/conversations?session_id=${session.session_id}&action=delete_non_favorites`, { method: 'DELETE' })
      .then(() => fetchConversations())
      .catch(() => {})
  }

  const toggleFavorite = (index: number) => {
    fetch(`/api/conversations?session_id=${session.session_id}&index=${index}`, { method: 'PATCH' })
      .then(r => r.json())
      .then((data: { favorite: boolean }) => {
        setSavedOutputs(prev => prev.map((item, i) => i === index ? { ...item, favorite: data.favorite } : item))
      })
      .catch(() => {})
  }

  const updateMemo = (text: string) => {
    setMemo(text)
    fetch(`/api/sessions/memo?id=${encodeURIComponent(session.session_id)}&memo=${encodeURIComponent(text)}`, { method: 'POST' })
  }

  useEffect(() => {
    fetch('/api/presets').then(r => r.json()).then(setPresets).catch(() => {})
    fetch('/api/config').then(r => r.json()).then((cfg: any) => {
      const bases: string[] = []
      for (const key of ['repository_base', 'worktree_base']) {
        if (cfg[key]) bases.push(cfg[key])
      }
      setCwdBases(bases)
      if (cfg.conversation?.height) setConversationHeight(cfg.conversation.height)
      if (cfg.conversation?.content_height) setContentHeight(cfg.conversation.content_height)
      if (cfg.preview_interval) setPreviewInterval(cfg.preview_interval * 1000)
    }).catch(() => {})
  }, [])

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

  const fetchScreen = async () => {
    setScreenLoading(true)
    try {
      const res = await fetch(`/api/sessions/screen?id=${session.session_id}`)
      const data = await res.json()
      setScreenContent(data.content || '')
    } catch { setScreenContent('') }
    setScreenLoading(false)
  }

  // Auto-fetch terminal preview + polling (except idle)
  useEffect(() => {
    if (!session.tty || session.status === 'idle') return
    fetchScreen()
    const id = setInterval(fetchScreen, previewInterval)
    return () => clearInterval(id)
  }, [session.session_id, session.tty, session.status, previewInterval])

  const displayCwd = (() => {
    for (const base of cwdBases) {
      const expanded = base.replace(/^~/, session.cwd.match(/^\/Users\/[^/]+/)?.[0] || '')
      if (session.cwd.startsWith(expanded + '/')) {
        return session.cwd.slice(expanded.length + 1)
      }
    }
    return session.cwd
  })()
  const displayName = session.custom_name || session.project_name

  const mdComponents = {
    code({ className, children }: { className?: string; children?: React.ReactNode }) {
      const code = String(children).trim()
      if (className === 'language-mermaid') {
        return <MermaidBlock code={code} light={outputLight} />
      }
      const lang = className?.replace('language-', '')
      if (lang) {
        return <ShikiHighlighter language={lang} theme={outputLight ? 'github-light' : 'github-dark'}>{code}</ShikiHighlighter>
      }
      return <code className={className}>{children}</code>
    }
  }

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
        <span className={`detail-status ${config.className}${session.status === 'ai_working' ? ' status-loading' : ''}${['waiting_input', 'permission_required'].includes(session.status) ? ' status-attention' : ''}`}>
          {session.status === 'ai_working' && <span className="loading-dots" />}
          {config.icon} {config.label}
        </span>
      </div>

      <div className="detail-actions">
        {session.status !== 'terminated' && (session.pid > 0 || session.tty) && (
          <button className="action-btn focus-btn" onClick={handleFocus} disabled={focusing}>
            {focusing ? 'Focusing...' : '🖥 Focus Terminal'}
          </button>
        )}
        <div className="detail-field-inline">
          <span className="detail-label">CWD</span>
          <span className="detail-value detail-mono">{displayCwd}</span>
        </div>
        <div className="detail-field-inline">
          <span className="detail-label">TTY</span>
          {editingTTY ? (
            <input
              ref={ttyInputRef}
              className="detail-name-input"
              style={{ fontSize: 14 }}
              value={editTTY}
              placeholder={t('tty_placeholder', locale)}
              onChange={e => setEditTTY(e.target.value)}
              onBlur={submitTTY}
              onKeyDown={e => { if (e.key === 'Enter') submitTTY(); if (e.key === 'Escape') setEditingTTY(false) }}
            />
          ) : (
            <span className="detail-value detail-mono" onClick={startEditTTY} style={{ cursor: 'pointer' }}>
              {session.tty || <span style={{ color: '#6c7086' }}>{t('click_to_set_tty', locale)}</span>}
              <span className="edit-icon" style={{ opacity: 0.5, marginLeft: 6 }}>✎</span>
            </span>
          )}
        </div>
      </div>

      {session.status === 'permission_required' && (session.pid > 0 || session.tty) && (
        <div className="detail-input-section">
          <div className="detail-input-label">{t('send_choice', locale)}</div>
          {keysSent ? (
            <div className="keys-sent-msg">{t('sent', locale)}</div>
          ) : (
            <div className="detail-input-row">
              <button className="action-btn" onClick={() => handleSendKeys('1')} disabled={sending}>1</button>
              <button className="action-btn" onClick={() => handleSendKeys('2')} disabled={sending}>2</button>
              <button className="action-btn" onClick={() => handleSendKeys('3')} disabled={sending}>3</button>
              <button className="action-btn" onClick={() => handleSendKeys('4')} disabled={sending}>4</button>
            </div>
          )}
        </div>
      )}

      {session.tty && session.status !== 'idle' && (
        <div className="detail-question">
          <div className="detail-question-label">
            Terminal Preview
            <button className="screen-refresh-btn" onClick={fetchScreen} disabled={screenLoading}>
              {screenLoading ? '...' : '↻'}
            </button>
          </div>
          <pre className="screen-content">{screenContent || (screenLoading ? t('loading', locale) : t('no_content', locale))}</pre>
        </div>
      )}

      {['idle', 'waiting_input'].includes(session.status) && (session.pid > 0 || session.tty) && (
        <div className="detail-input-section">
          <div className="detail-input-label">{t('presets', locale)}</div>
          <div className="detail-input-row">
            {presets.map((p, i) => (
              <button key={i} className="action-btn" onClick={() => handleSendInput(p.text)} disabled={sending}>{p.label}</button>
            ))}
          </div>
          <div className="detail-input-label" style={{ marginTop: 12 }}>{t('free_input', locale)}</div>
          <div className="detail-input-row">
            <textarea
              className="detail-send-input detail-send-textarea"
              value={sendText}
              placeholder={t('type_to_send', locale)}
              onChange={e => setSendText(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter' && e.metaKey && sendText.trim()) { e.preventDefault(); handleSendInput(sendText.trim()) } }}
              disabled={sending}
              rows={2}
            />
            <button className="action-btn" onClick={() => handleSendInput(sendText.trim())} disabled={sending || !sendText.trim()}>
              {t('send_shortcut', locale)}
            </button>
          </div>
        </div>
      )}

      <div className="detail-question">
        <div className="detail-question-label memo-label" onClick={() => setMemoOpen(v => !v)} style={{ cursor: 'pointer' }}>
          {memoOpen ? '▾' : '▸'} Memo
          {!memoOpen && memo && <span className="memo-has-content">●</span>}
        </div>
        {memoOpen && (
          <textarea
            className="memo-textarea"
            value={memo}
            onChange={e => updateMemo(e.target.value)}
            placeholder={t('enter_memo', locale)}
            rows={4}
          />
        )}
      </div>


      {session.question && (
        <div className="detail-question">
          <div className="detail-question-label">Question</div>
          <div className="detail-question-text">{session.question}</div>
        </div>
      )}

      <div className="history-section">
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <button className="history-toggle" style={{ flex: 1 }} onClick={() => setSavedOpen(v => !v)}>
            {savedOpen ? '▾' : '▸'} Conversation
            {savedOutputs.length > 1 && <span className="history-count">{savedOutputs.length - 1}</span>}
          </button>
          <button
            className="action-btn"
            style={{ fontSize: '11px', padding: '1px 6px', minWidth: 0, opacity: favFilter ? 1 : 0.5 }}
            onClick={() => setFavFilter(v => !v)}
            title="Filter favorites"
          >★</button>
          {savedOutputs.some(s => s.favorite) && (
            <button
              className="action-btn deny-btn"
              style={{ fontSize: '10px', padding: '1px 4px', minWidth: 0 }}
              onClick={() => { if (confirm(t('confirm_delete_non_favorites', locale))) deleteNonFavorites() }}
              title="Delete non-favorites"
            >🗑️</button>
          )}
          <button
            className="action-btn"
            style={{ fontSize: '11px', padding: '1px 6px', minWidth: 0 }}
            onClick={() => setOutputMode(m => { const next = m === 'text' ? 'md' : 'text'; localStorage.setItem('output-mode', next); return next })}
          >{outputMode === 'text' ? 'MD' : 'Text'}</button>
          <button
            className="action-btn"
            style={{ fontSize: '11px', padding: '1px 6px', minWidth: 0 }}
            onClick={() => setOutputLight(v => { const next = !v; localStorage.setItem('output-light', next ? '1' : '0'); return next })}
          >{outputLight ? '🌙' : '☀️'}</button>
        </div>
        <div className="history-list" style={{ maxHeight: conversationHeight }}>
          {savedOutputs.map((item, i) => {
            if (i > 0 && !savedOpen) return null
            if (favFilter && !item.favorite) return null
            const expanded = expandedSaved.has(i)
            return (
              <div key={i} className="history-msg history-assistant">
                <div className="history-role" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', cursor: 'pointer' }}
                  onClick={() => setExpandedSaved(prev => {
                    const next = new Set(prev)
                    next.has(i) ? next.delete(i) : next.add(i)
                    return next
                  })}>
                  <span>
                    {expanded ? '▾' : '▸'} {i === 0 ? 'Latest' : item.savedAt}
                    <button
                      className="action-btn"
                      style={{ fontSize: '10px', padding: '0px 4px', minWidth: 0, marginLeft: 6, opacity: item.favorite ? 1 : 0.3 }}
                      onClick={(e) => { e.stopPropagation(); toggleFavorite(i) }}
                    >{item.favorite ? '★' : '☆'}</button>
                  </span>
                  {i > 0 && (
                    <button
                      className="action-btn deny-btn"
                      style={{ fontSize: '10px', padding: '0px 4px', minWidth: 0 }}
                      onClick={(e) => { e.stopPropagation(); deleteSavedOutput(i) }}
                    >x</button>
                  )}
                </div>
                {expanded && (
                  <>
                    {item.input && (
                      <div style={{ marginBottom: '4px' }}>
                        <div className="history-role" style={{ color: '#89b4fa' }}>User Input</div>
                        <div className="history-content">{item.input}</div>
                      </div>
                    )}
                    <div>
                      <div className="history-role" style={{ color: '#a6e3a1', display: 'flex', alignItems: 'center', gap: '6px' }}>
                        Output
                        <button
                          className="action-btn"
                          style={{ fontSize: '10px', padding: '0px 4px', minWidth: 0 }}
                          onClick={() => navigator.clipboard.writeText(item.output)}
                        >⧉</button>
                      </div>
                      <div className={`history-content${outputLight ? ' output-light' : ''}`} style={{ maxHeight: 'none' }}>
                        {outputMode === 'md'
                          ? <Markdown remarkPlugins={[remarkGfm]} components={mdComponents}>{item.output}</Markdown>
                          : item.output}
                      </div>
                    </div>
                  </>
                )}
              </div>
            )
          })}
        </div>
      </div>

      <div className="history-section">
        <button className="history-toggle" onClick={() => setInfoOpen(v => !v)}>
          {infoOpen ? '▾' : '▸'} Info
        </button>
        {infoOpen && (
          <div className="detail-grid">
            <div className="detail-field">
              <span className="detail-label">PID</span>
              <span className="detail-value detail-mono">{session.pid || '-'}</span>
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
            <div className="detail-field">
              <span className="detail-label">Conversation File</span>
              <span className="detail-value detail-mono" style={{ fontSize: '11px' }}>~/.claude-tabs/conversations/{session.session_id}.json</span>
            </div>
          </div>
        )}
      </div>



      <div className="history-section">
        <button className="history-toggle" onClick={() => setHistoryOpen(v => !v)}>
          {historyOpen ? '▾' : '▸'} Conversation History
          {history.length > 0 && <span className="history-count">{history.length}</span>}
        </button>
        {historyOpen && (
          <div className="history-list">
            {history.length === 0 && (
              <div className="history-empty">{t('no_history', locale)}</div>
            )}
            {history.map((msg, i) => (
              <div key={i} className={`history-msg history-${msg.role}`}>
                <div className="history-role">{msg.role === 'user' ? 'You' : 'Claude'}</div>
                <div className="history-content" style={{ maxHeight: contentHeight }}>{msg.content}</div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
