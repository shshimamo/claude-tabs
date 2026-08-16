import { useState, useEffect, useMemo } from 'react'
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

type Mode = 'existing' | 'worktree'

export default function SbxRunModal({ onClose, locale, onCreateProject }: Props) {
  const [sbxList, setSbxList] = useState<string[]>([])
  const [repoList, setRepoList] = useState<string[]>([])
  const [sbx, setSbx] = useState('')
  const [mode, setMode] = useState<Mode>('existing')
  // existing mode
  const [repoFilter, setRepoFilter] = useState('')
  const [selectedRepo, setSelectedRepo] = useState('')
  // worktree mode
  const [wtRepo, setWtRepo] = useState('')
  const [wtBranch, setWtBranch] = useState('')
  const [wtBase, setWtBase] = useState('')
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)
  // project creation
  const [createProject, setCreateProject] = useState(false)
  const [projectName, setProjectName] = useState('')
  const [githubUrl, setGithubUrl] = useState('')
  const [slackUrl, setSlackUrl] = useState('')

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
            </>
          ) : (
            <>
              <label className="modal-label">Repository</label>
              <input
                className="modal-input"
                value={wtRepo}
                onChange={e => setWtRepo(e.target.value)}
                placeholder="e.g. claude-tabs"
              />
              <label className="modal-label">
                Branch / PR Link
                <span className="tooltip-wrap">
                  <span className="tooltip-icon">?</span>
                  <span className="tooltip-content">
                    {t('branch_tooltip', locale).split('\n').map((line, i) => <span key={i}>{line}{'\n'}</span>)}
                  </span>
                </span>
              </label>
              <input
                className="modal-input"
                value={wtBranch}
                onChange={e => setWtBranch(e.target.value)}
                placeholder="e.g. feature/xxx or https://github.com/.../pull/123"
              />
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
