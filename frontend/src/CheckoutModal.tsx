import { useState, useEffect, useRef } from 'react'

type Props = {
  onClose: () => void
}

type RepoInfo = {
  name: string
  path: string
  branch: string
}

export default function CheckoutModal({ onClose }: Props) {
  const [repos, setRepos] = useState<RepoInfo[]>([])
  const [selectedRepo, setSelectedRepo] = useState<string>('')
  const [branch, setBranch] = useState('')
  const [branches, setBranches] = useState<string[]>([])
  const [filteredBranches, setFilteredBranches] = useState<string[]>([])
  const [showSuggestions, setShowSuggestions] = useState(false)
  const [loading, setLoading] = useState(false)
  const [branchLoading, setBranchLoading] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    fetch('/api/git/repos').then(r => r.json()).then(setRepos).catch(() => {})
  }, [])

  const handleRepoSelect = (repoName: string) => {
    setSelectedRepo(repoName)
    setBranch('')
    setBranches([])
    setResult(null)
  }

  const handleBranchFocus = () => {
    if (branches.length === 0 && selectedRepo) {
      setBranchLoading(true)
      fetch(`/api/git/branches?repo=${encodeURIComponent(selectedRepo)}`)
        .then(r => r.json())
        .then((data: string[]) => {
          setBranches(data)
          setFilteredBranches(data)
          setShowSuggestions(true)
        })
        .catch(() => {})
        .finally(() => setBranchLoading(false))
    } else {
      setFilteredBranches(branches.filter(b => b.includes(branch)))
      setShowSuggestions(true)
    }
  }

  const handleBranchChange = (value: string) => {
    setBranch(value)
    setFilteredBranches(branches.filter(b => b.toLowerCase().includes(value.toLowerCase())))
    setShowSuggestions(true)
  }

  const handleBranchSelect = (b: string) => {
    setBranch(b)
    setShowSuggestions(false)
    inputRef.current?.focus()
  }

  const handleCheckout = async () => {
    if (!selectedRepo) return
    setLoading(true)
    setResult(null)
    try {
      const params = new URLSearchParams({ repo: selectedRepo })
      if (branch.trim()) params.set('branch', branch.trim())
      const res = await fetch(`/api/git/checkout?${params}`, { method: 'POST' })
      const data = await res.json()
      setResult({ ok: res.ok, message: data.message })
      if (res.ok) {
        // update repo list
        fetch('/api/git/repos').then(r => r.json()).then(setRepos).catch(() => {})
      }
    } catch {
      setResult({ ok: false, message: 'Request failed' })
    }
    setLoading(false)
  }

  const currentBranch = repos.find(r => r.name === selectedRepo)?.branch || ''

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal wt-modal" style={{ maxWidth: 600 }} onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Checkout & Pull</h3>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="modal-body">
          <label className="modal-label">Repository</label>
          <div className="checkout-repo-list">
            {repos.map(r => (
              <div
                key={r.name}
                className={`checkout-repo-item${selectedRepo === r.name ? ' selected' : ''}`}
                onClick={() => handleRepoSelect(r.name)}
              >
                <span className="checkout-repo-name">{r.name}</span>
                <span className="checkout-repo-branch">{r.branch}</span>
              </div>
            ))}
            {repos.length === 0 && <div className="checkout-repo-item">No repositories found</div>}
          </div>

          {selectedRepo && (
            <>
              <label className="modal-label" style={{ marginTop: 12 }}>
                Branch <span style={{ opacity: 0.5, fontSize: '0.85em' }}>({currentBranch})</span>
              </label>
              <div style={{ position: 'relative' }}>
                <input
                  ref={inputRef}
                  className="modal-input"
                  value={branch}
                  onChange={e => handleBranchChange(e.target.value)}
                  onFocus={handleBranchFocus}
                  onBlur={() => setTimeout(() => setShowSuggestions(false), 200)}
                  onKeyDown={e => { if (e.key === 'Enter') handleCheckout() }}
                  placeholder={branchLoading ? 'Loading branches...' : `empty = pull ${currentBranch}`}
                />
                {showSuggestions && filteredBranches.length > 0 && (
                  <div className="checkout-suggestions">
                    {filteredBranches.slice(0, 20).map(b => (
                      <div
                        key={b}
                        className="checkout-suggestion-item"
                        onMouseDown={() => handleBranchSelect(b)}
                      >
                        {b}
                      </div>
                    ))}
                    {filteredBranches.length > 20 && (
                      <div className="checkout-suggestion-item" style={{ opacity: 0.5 }}>
                        ...{filteredBranches.length - 20} more
                      </div>
                    )}
                  </div>
                )}
              </div>
            </>
          )}

          {result && (
            <div className={`modal-result ${result.ok ? 'modal-result-ok' : 'modal-result-err'}`}>
              {result.message}
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button className="action-btn" onClick={onClose} disabled={loading}>Cancel</button>
          <button
            className="action-btn allow-btn"
            onClick={handleCheckout}
            disabled={loading || !selectedRepo}
          >
            {loading ? 'Running...' : branch.trim() ? 'Switch & Pull' : 'Pull'}
          </button>
        </div>
      </div>
    </div>
  )
}
