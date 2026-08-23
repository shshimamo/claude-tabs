import { useState, useEffect } from 'react'

type Props = {
  onClose: () => void
}

export default function DockerfileModal({ onClose }: Props) {
  const [content, setContent] = useState('')
  const [isDefault, setIsDefault] = useState(false)
  const [saving, setSaving] = useState(false)
  const [building, setBuilding] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)

  useEffect(() => {
    fetch('/api/sbx/dockerfile').then(r => r.json()).then(data => {
      setContent(data.content || '')
      setIsDefault(data.is_default === 'true')
    }).catch(() => {})
  }, [])

  const handleSave = async () => {
    setSaving(true)
    setResult(null)
    try {
      const res = await fetch('/api/sbx/dockerfile', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content }),
      })
      const data = await res.json()
      setResult({ ok: res.ok, message: data.message })
      if (res.ok) setIsDefault(false)
    } catch {
      setResult({ ok: false, message: 'Save failed' })
    }
    setSaving(false)
  }

  const handleBuild = async () => {
    if (isDefault) {
      // save first if using default
      await handleSave()
    }
    setBuilding(true)
    setResult(null)
    try {
      const res = await fetch('/api/sbx/build-template', { method: 'POST' })
      const data = await res.json()
      setResult({ ok: res.ok, message: data.message })
    } catch {
      setResult({ ok: false, message: 'Build failed' })
    }
    setBuilding(false)
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal wt-modal" style={{ maxWidth: 700 }} onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Dockerfile Template</h3>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="modal-body">
          {isDefault && (
            <div className="modal-result modal-result-ok" style={{ marginBottom: 8 }}>
              Default template shown. Edit and save to customize.
            </div>
          )}
          <textarea
            className="dockerfile-editor"
            value={content}
            onChange={e => setContent(e.target.value)}
            spellCheck={false}
          />
          {result && (
            <div className={`modal-result ${result.ok ? 'modal-result-ok' : 'modal-result-err'}`}>
              {result.message}
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button className="action-btn" onClick={onClose}>Close</button>
          <button className="action-btn" onClick={handleSave} disabled={saving}>
            {saving ? 'Saving...' : 'Save'}
          </button>
          <button className="action-btn allow-btn" onClick={handleBuild} disabled={building || saving}>
            {building ? 'Building...' : 'Build Template'}
          </button>
        </div>
      </div>
    </div>
  )
}
