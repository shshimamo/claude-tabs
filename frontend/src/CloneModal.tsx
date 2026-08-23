import { useState } from 'react'

type Props = {
  onClose: () => void
}

export default function CloneModal({ onClose }: Props) {
  const [url, setUrl] = useState('')
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)

  const handleClone = async () => {
    if (!url.trim()) return
    setRunning(true)
    setResult(null)
    try {
      const res = await fetch(`/api/git/clone?url=${encodeURIComponent(url.trim())}`, { method: 'POST' })
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
          <h3>Git Clone</h3>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="modal-body">
          <label className="modal-label">Repository URL</label>
          <input
            className="modal-input"
            value={url}
            onChange={e => setUrl(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter' && url.trim()) handleClone() }}
            placeholder="e.g. https://github.com/org/repo"
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
          <button className="action-btn allow-btn" onClick={handleClone} disabled={running || !url.trim()}>
            {running ? 'Cloning...' : 'Clone'}
          </button>
        </div>
      </div>
    </div>
  )
}
