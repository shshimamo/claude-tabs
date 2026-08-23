import { useState } from 'react'

type Props = {
  onClose: () => void
}

export default function CreateSbxModal({ onClose }: Props) {
  const [name, setName] = useState('')
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)

  const handleCreate = async () => {
    if (!name.trim()) return
    setRunning(true)
    setResult(null)
    try {
      const res = await fetch(`/api/sbx/create?name=${encodeURIComponent(name.trim())}`, { method: 'POST' })
      const data = await res.json()
      setResult({ ok: res.ok, message: data.message })
      if (res.ok) setTimeout(onClose, 2000)
    } catch {
      setResult({ ok: false, message: 'Request failed' })
    }
    setRunning(false)
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal wt-modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Create sbx</h3>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="modal-body">
          <label className="modal-label">Sandbox Name</label>
          <input
            className="modal-input"
            value={name}
            onChange={e => setName(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter' && name.trim()) handleCreate() }}
            placeholder="e.g. my-review"
            autoFocus
          />
          {result && (
            <div className={`modal-result ${result.ok ? 'modal-result-ok' : 'modal-result-err'}`}>
              {result.message}
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button className="action-btn" onClick={onClose} disabled={running}>Cancel</button>
          <button className="action-btn allow-btn" onClick={handleCreate} disabled={running || !name.trim()}>
            {running ? 'Creating...' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  )
}
