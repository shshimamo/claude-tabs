import { useState } from 'react'

type Props = {
  onClose: () => void
}

export default function WorktreeModal({ onClose }: Props) {
  const [repo, setRepo] = useState('')
  const [branch, setBranch] = useState('')
  const [creating, setCreating] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)

  const handleCreate = async () => {
    if (!repo.trim() || !branch.trim()) return
    setCreating(true)
    setResult(null)
    try {
      const res = await fetch(`/api/worktree/create?repo=${encodeURIComponent(repo.trim())}&branch=${encodeURIComponent(branch.trim())}`, { method: 'POST' })
      const data = await res.json()
      setResult({ ok: res.ok, message: data.message })
      if (res.ok) {
        setTimeout(onClose, 2000)
      }
    } catch {
      setResult({ ok: false, message: 'Request failed' })
    }
    setCreating(false)
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3>New Worktree + Claude</h3>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="modal-body">
          <label className="modal-label">Repository</label>
          <input
            className="modal-input"
            value={repo}
            onChange={e => setRepo(e.target.value)}
            placeholder="e.g. claude-tabs"
            autoFocus
          />
          <label className="modal-label">Branch</label>
          <input
            className="modal-input"
            value={branch}
            onChange={e => setBranch(e.target.value)}
            placeholder="e.g. feature/new-feature"
            onKeyDown={e => { if (e.key === 'Enter' && repo.trim() && branch.trim()) handleCreate() }}
          />
          {result && (
            <div className={`modal-result ${result.ok ? 'modal-result-ok' : 'modal-result-err'}`}>
              {result.message}
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button className="action-btn" onClick={onClose} disabled={creating}>Cancel</button>
          <button className="action-btn allow-btn" onClick={handleCreate} disabled={creating || !repo.trim() || !branch.trim()}>
            {creating ? 'Creating...' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  )
}
