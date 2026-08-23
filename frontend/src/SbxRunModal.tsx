import { useState, useEffect, useMemo, useRef } from 'react'
import { t, type Locale } from './i18n'

type CreatedProject = {
  name: string
  githubUrl: string
  slackUrl: string
}

type Props = {
  onClose: () => void
  locale: Locale
  onCreateProject?: (project: CreatedProject) => Promise<void>
}

type RepoInfo = { path: string; branch: string }
type BranchInfo = { name: string; pr?: number }
type Mode = 'existing' | 'worktree'

export default function SbxRunModal({ onClose, locale, onCreateProject }: Props) {
  const [sbxList, setSbxList] = useState<string[]>([])
  const [repoList, setRepoList] = useState<RepoInfo[]>([])
  const [sbx, setSbx] = useState('')
  const [mode, setMode] = useState<Mode>('existing')
  // existing mode
  const [repoFilter, setRepoFilter] = useState('')
  const [selectedRepo, setSelectedRepo] = useState('')
  const [showRepoList, setShowRepoList] = useState(false)
  // worktree mode
  const [wtRepo, setWtRepo] = useState('')
  const [wtRepoFilter, setWtRepoFilter] = useState('')
  const [showWtRepoList, setShowWtRepoList] = useState(false)
  const [wtBranch, setWtBranch] = useState('')
  const [wtBase, setWtBase] = useState('')
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)
  // branch suggestions
  const [branchList, setBranchList] = useState<BranchInfo[]>([])
  const [branchLoading, setBranchLoading] = useState(false)
  const [showBranchList, setShowBranchList] = useState(false)
  const branchRef = useRef<HTMLInputElement>(null)
  // project creation
  const [createProject, setCreateProject] = useState(false)
  const [projectName, setProjectName] = useState('')
  const [githubUrl, setGithubUrl] = useState('')
  const [slackUrl, setSlackUrl] = useState('')

  useEffect(() => {
    fetch('/api/sbx/list').then(r => r.json()).then(setSbxList).catch(() => {})
  }, [])

  useEffect(() => {
    if (!sbx) { setRepoList([]); return }
    setRepoList([])
    setSelectedRepo('')
    setRepoFilter('')
    setWtRepo('')
    setWtRepoFilter('')
    setBranchList([])
    fetch(`/api/sbx/repos?sbx=${encodeURIComponent(sbx)}`).then(r => r.json()).then(setRepoList).catch(() => {})
  }, [sbx])

  const filteredRepos = useMemo(() => {
    if (!repoFilter.trim()) return repoList
    const q = repoFilter.trim().toLowerCase()
    return repoList.filter(r => r.path.toLowerCase().includes(q))
  }, [repoList, repoFilter])

  const filteredWtRepos = useMemo(() => {
    if (!wtRepoFilter.trim()) return repoList
    const q = wtRepoFilter.trim().toLowerCase()
    return repoList.filter(r => r.path.toLowerCase().includes(q))
  }, [repoList, wtRepoFilter])

  const filteredBranches = useMemo(() => {
    if (!wtBranch.trim()) return branchList
    const q = wtBranch.trim().toLowerCase()
    return branchList.filter(b => b.name.toLowerCase().includes(q))
  }, [branchList, wtBranch])

  const handleBranchFocus = () => {
    if (branchList.length === 0 && sbx && wtRepo) {
      setBranchLoading(true)
      fetch(`/api/sbx/branches?sbx=${encodeURIComponent(sbx)}&repo=${encodeURIComponent(wtRepo)}`)
        .then(r => r.json())
        .then((data: BranchInfo[]) => { setBranchList(data); setShowBranchList(true) })
        .catch(() => {})
        .finally(() => setBranchLoading(false))
    } else {
      setShowBranchList(true)
    }
  }

  const handleRun = async () => {
    if (!sbx) return
    setRunning(true)
    setResult(null)
    try {
      let res: Response
      if (mode === 'existing') {
        if (!selectedRepo) return
        res = await fetch(`/api/sbx/run?sbx=${encodeURIComponent(sbx)}&repo=${encodeURIComponent(selectedRepo)}`, { method: 'POST' })
      } else {
        if (!wtRepo || !wtBranch) return
        const params = new URLSearchParams({ sbx, repo: wtRepo, branch: wtBranch })
        if (wtBase.trim()) params.set('base', wtBase.trim())
        res = await fetch(`/api/sbx/attach-worktree?${params}`, { method: 'POST' })
      }
      const data = await res.json()
      setResult({ ok: res.ok, message: data.message })
      if (res.ok && createProject && projectName.trim() && onCreateProject) {
        await onCreateProject({ name: projectName.trim(), githubUrl: githubUrl.trim(), slackUrl: slackUrl.trim() })
      }
      if (res.ok) setTimeout(onClose, 2000)
    } catch {
      setResult({ ok: false, message: 'Request failed' })
    }
    setRunning(false)
  }

  const canRun = sbx && (mode === 'existing' ? selectedRepo : wtRepo && wtBranch)

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal wt-modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Attach sbx</h3>
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

          <div className="mode-tabs">
            <button
              className={`mode-tab${mode === 'existing' ? ' mode-tab-active' : ''}`}
              onClick={() => setMode('existing')}
            >
              Existing repo
            </button>
            <button
              className={`mode-tab${mode === 'worktree' ? ' mode-tab-active' : ''}`}
              onClick={() => setMode('worktree')}
            >
              New worktree
            </button>
          </div>

          {mode === 'existing' ? (
            <>
              <label className="modal-label">Repository</label>
              <input
                className="modal-input"
                value={repoFilter}
                onChange={e => { setRepoFilter(e.target.value); setSelectedRepo(''); setShowRepoList(true) }}
                onFocus={() => setShowRepoList(true)}
                onBlur={() => setTimeout(() => setShowRepoList(false), 200)}
                placeholder="Filter repositories..."
              />
              {filteredRepos.length > 0 && !selectedRepo && showRepoList && (
                <div className="repo-list">
                  {filteredRepos.map(r => (
                    <div
                      key={r.path}
                      className="repo-item"
                      onClick={() => { setSelectedRepo(r.path); setRepoFilter(r.path) }}
                    >
                      <span>{r.path}</span>
                      {r.branch && <span className="checkout-repo-branch">{r.branch}</span>}
                    </div>
                  ))}
                </div>
              )}
            </>
          ) : (
            <>
              <label className="modal-label">Repository</label>
              <input
                className="modal-input"
                value={wtRepoFilter}
                onChange={e => { setWtRepoFilter(e.target.value); setWtRepo(''); setShowWtRepoList(true) }}
                onFocus={() => setShowWtRepoList(true)}
                onBlur={() => setTimeout(() => setShowWtRepoList(false), 200)}
                placeholder="Filter repositories..."
              />
              {filteredWtRepos.length > 0 && !wtRepo && showWtRepoList && (
                <div className="repo-list">
                  {filteredWtRepos.map(r => (
                    <div
                      key={r.path}
                      className="repo-item"
                      onClick={() => { setWtRepo(r.path); setWtRepoFilter(r.path); setBranchList([]); setWtBranch('') }}
                    >
                      <span>{r.path}</span>
                      {r.branch && <span className="checkout-repo-branch">{r.branch}</span>}
                    </div>
                  ))}
                </div>
              )}
              <label className="modal-label">
                Branch / PR Link
                <span className="tooltip-wrap">
                  <span className="tooltip-icon">?</span>
                  <span className="tooltip-content">
                    {t('branch_tooltip', locale).split('\n').map((line, i) => <span key={i}>{line}{'\n'}</span>)}
                  </span>
                </span>
              </label>
              <div style={{ position: 'relative' }}>
                <input
                  ref={branchRef}
                  className="modal-input"
                  value={wtBranch}
                  onChange={e => { setWtBranch(e.target.value); setShowBranchList(true) }}
                  onFocus={handleBranchFocus}
                  onBlur={() => setTimeout(() => setShowBranchList(false), 200)}
                  placeholder={branchLoading ? 'Loading branches...' : 'e.g. feature/xxx or https://github.com/.../pull/123'}
                />
                {showBranchList && filteredBranches.length > 0 && (
                  <div className="checkout-suggestions">
                    {filteredBranches.slice(0, 20).map(b => (
                      <div
                        key={b.name}
                        className="checkout-suggestion-item"
                        onMouseDown={() => { setWtBranch(b.name); setShowBranchList(false); branchRef.current?.focus() }}
                      >
                        <span>{b.name}</span>
                        {b.pr ? <span className="checkout-repo-branch">PR#{b.pr}</span> : null}
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
              <label className="modal-label">Base Branch <span style={{ color: '#6c7086', fontSize: 12 }}>{t('base_branch_hint', locale)}</span></label>
              <input
                className="modal-input"
                value={wtBase}
                onChange={e => setWtBase(e.target.value)}
                placeholder="e.g. main, develop"
              />
            </>
          )}

          <div className="modal-project-toggle">
            <label>
              <input
                type="checkbox"
                checked={createProject}
                onChange={e => setCreateProject(e.target.checked)}
              />
              {' '}Create Project
            </label>
          </div>
          {createProject && (
            <div className="modal-project-fields">
              <label className="modal-label">Project Name</label>
              <input
                className="modal-input"
                value={projectName}
                onChange={e => setProjectName(e.target.value)}
                placeholder="e.g. Review PR #123"
              />
              <label className="modal-label">GitHub Link <span style={{ color: '#6c7086', fontSize: 12 }}>optional</span></label>
              <input
                className="modal-input"
                value={githubUrl}
                onChange={e => setGithubUrl(e.target.value)}
                placeholder="e.g. https://github.com/org/repo/pull/123"
              />
              <label className="modal-label">Slack Link <span style={{ color: '#6c7086', fontSize: 12 }}>optional</span></label>
              <input
                className="modal-input"
                value={slackUrl}
                onChange={e => setSlackUrl(e.target.value)}
                placeholder="e.g. https://org.slack.com/archives/..."
              />
            </div>
          )}

          {sbx && mode === 'existing' && selectedRepo && (
            <div className="modal-steps">
              <div className="modal-steps-label">{t('commands', locale)}</div>
              <div className="modal-step">sbx exec -it {sbx} sh -c 'cd {selectedRepo} && claude'</div>
            </div>
          )}

          {sbx && mode === 'worktree' && wtRepo && wtBranch && (
            <div className="modal-steps">
              <div className="modal-steps-label">{t('commands', locale)}</div>
              <div className="modal-step">git worktree add ~/worktrees/{wtRepo}/{wtBranch}{wtBase ? ` (base: ${wtBase})` : ''}</div>
              <div className="modal-step">sbx exec -it {sbx} sh -c 'cd .../{wtRepo}/{wtBranch} && claude'</div>
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
          <button className="action-btn allow-btn" onClick={handleRun} disabled={running || !canRun}>
            {running ? 'Starting...' : 'Run'}
          </button>
        </div>
      </div>
    </div>
  )
}
