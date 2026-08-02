import { useState, useEffect } from 'react'

type Props = {
  onClose: () => void
}

export default function ConfigModal({ onClose }: Props) {
  const [text, setText] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    fetch('/api/config')
      .then(r => r.text())
      .then(t => {
        try { setText(JSON.stringify(JSON.parse(t), null, 2)) } catch { setText(t) }
        setLoading(false)
      })
  }, [])

  const handleSave = async () => {
    setError('')
    setSaved(false)
    try {
      JSON.parse(text)
    } catch {
      setError('Invalid JSON')
      return
    }
    setSaving(true)
    const res = await fetch('/api/config', { method: 'PUT', body: text })
    setSaving(false)
    if (!res.ok) {
      setError(await res.text())
      return
    }
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal config-modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Settings</h3>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="modal-body">
          {loading ? (
            <div>Loading...</div>
          ) : (
            <textarea
              className="config-editor"
              value={text}
              onChange={e => setText(e.target.value)}
              spellCheck={false}
            />
          )}
          {error && <div className="modal-result modal-result-err">{error}</div>}
          {saved && <div className="modal-result modal-result-ok">Saved</div>}
        </div>
        <div className="modal-footer">
          <button className="action-btn" onClick={onClose}>Close</button>
          <button className="action-btn allow-btn" onClick={handleSave} disabled={saving || loading}>
            {saving ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  )
}
