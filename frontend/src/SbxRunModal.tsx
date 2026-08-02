import { useState, useEffect, useMemo } from 'react'

type Props = {
  onClose: () => void
}

export default function SbxRunModal({ onClose }: Props) {
  const [sbxList, setSbxList] = useState<string[]>([])
  const [repoList, setRepoList] = useState<string[]>([])
  const [sbx, setSbx] = useState('')
  const [repoFilter, setRepoFilter] = useState('')
  const [selectedRepo, setSelectedRepo] = useState('')
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)

  useEffect(() => {
    fetch('/api/sbx/list').then(r => r.json()).then(setSbxList).catch(() => {})
    fetch('/api/sbx/repos').then(r => r.json()).then(setRepoList).catch(() => {})
  }, [])

  const filteredRepos = useMemo(() => {
    if (!repoFilter.trim()) return repoList
    const q = repoFilter.trim().toLowerCase()
    return repoList.filter(r => r.toLowerCase().includes(q))
  }, [repoList, repoFilter])

  const handleRun = async () => {
    if (!sbx || !selectedRepo) return
    setRunning(true)
    setResult(null)
    try {
      const res = await fetch(`/api/sbx/run?sbx=${encodeURIComponent(sbx)}&repo=${encodeURIComponent(selectedRepo)}`, { method: 'POST' })
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
          <h3>Run Claude in sbx</h3>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="modal-body">
          <label className="modal-label">Sandbox</label>
          <select
            className="modal-input"
            value={sbx}
            onChange={e => setSbx(e.target.value)}
          >
            <option value="">-- select --</option>
            {sbxList.map(s => <option key={s} value={s}>{s}</option>)}
          </select>

          <label className="modal-label">Repository</label>
          <input
            className="modal-input"
            value={repoFilter}
            onChange={e => { setRepoFilter(e.target.value); setSelectedRepo('') }}
            placeholder="Filter repositories..."
          />
          {filteredRepos.length > 0 && !selectedRepo && (
            <div className="repo-list">
              {filteredRepos.map(r => (
                <div
                  key={r}
                  className="repo-item"
                  onClick={() => { setSelectedRepo(r); setRepoFilter(r) }}
                >
                  {r}
                </div>
              ))}
            </div>
          )}

          {sbx && selectedRepo && (
            <div className="modal-steps">
              <div className="modal-steps-label">実行コマンド</div>
              <div className="modal-step">sbx exec -it {sbx} sh -c 'cd {selectedRepo} && claude'</div>
            </div>
          )}

          {result && (
            <div className={`modal-result ${result.ok ? 'modal-result-ok' : 'modal-result-err'}`}>
              {result.message}
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button className="action-btn" onClick={onClose} disabled={running}>Cancel</button>
          <button className="action-btn allow-btn" onClick={handleRun} disabled={running || !sbx || !selectedRepo}>
            {running ? 'Starting...' : 'Run'}
          </button>
        </div>
      </div>
    </div>
  )
}
