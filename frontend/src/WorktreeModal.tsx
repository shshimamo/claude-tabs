import { useState, useEffect, useMemo } from 'react'

type Config = {
  worktree_base?: string
  sbx_template?: string
  sbx_default_mounts?: string[]
  sbx_kits?: string[]
  sbx_post_create_cmds?: string[][]
  plugins?: { source: string; plugins: string[] }[]
}

type Props = {
  onClose: () => void
}

export default function WorktreeModal({ onClose }: Props) {
  const [repo, setRepo] = useState('')
  const [branch, setBranch] = useState('')
  const [baseBranch, setBaseBranch] = useState('')
  const [creating, setCreating] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)
  const [cfg, setCfg] = useState<Config>({})

  useEffect(() => {
    fetch('/api/config').then(r => r.json()).then(setCfg).catch(() => {})
  }, [])

  const handleCreate = async () => {
    if (!repo.trim() || !branch.trim()) return
    setCreating(true)
    setResult(null)
    try {
      const params = new URLSearchParams({ repo: repo.trim(), branch: branch.trim() })
      if (baseBranch.trim()) params.set('base', baseBranch.trim())
      const res = await fetch(`/api/worktree/create?${params}`, { method: 'POST' })
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

  const r = repo.trim() || '<repo>'
  const b = branch.trim() || '<branch>'
  const sbxName = `wt-${r}-${b}`
  const wtBase = cfg.worktree_base || '~/worktrees'
  const template = cfg.sbx_template || 'my-sbx:latest'
  const wtPath = `${wtBase}/${r}/${b}`

  const steps = useMemo(() => {
    const s: string[] = []
    s.push(`git worktree add ${wtPath}${baseBranch.trim() ? ` (base: ${baseBranch.trim()})` : ''}`)
    const mounts = [wtPath, '~/.claude-tabs', ...(cfg.sbx_default_mounts || [])]
    const kits = (cfg.sbx_kits || []).map(k => `--kit ${k}`).join(' ')
    s.push(`sbx create --name ${sbxName} -t ${template}${kits ? ' ' + kits : ''} claude ${mounts.join(' ')}`)
    s.push(`sbx exec ${sbxName} ln -sf <host>/.claude-tabs ~/.claude-tabs`)
    for (const cmd of cfg.sbx_post_create_cmds || []) {
      s.push(`sbx exec ${sbxName} ${cmd.join(' ')}`)
    }
    for (const pc of cfg.plugins || []) {
      s.push(`sbx exec ${sbxName} claude plugins marketplace add ${pc.source}`)
      for (const p of pc.plugins) {
        if (p === 'auto') {
          s.push(`sbx exec ${sbxName} claude plugins install <auto-detected>`)
        } else {
          s.push(`sbx exec ${sbxName} claude plugins install ${p}`)
        }
      }
    }
    s.push(`sbx run --name ${sbxName} claude`)
    return s
  }, [r, b, cfg, wtPath, sbxName, template])

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal wt-modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3>New sbx claude</h3>
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
          <label className="modal-label">
            Branch
            <span className="tooltip-wrap">
              <span className="tooltip-icon">?</span>
              <span className="tooltip-content">
                1. worktree が既に存在 → そのまま使用{'\n'}
                2. origin/branch が存在 → リモートブランチから作成{'\n'}
                3. ローカルに branch が存在 → ローカルブランチから作成{'\n'}
                4. どちらもない → base branch (default: リポジトリのチェックアウト中ブランチ) から新規作成
              </span>
            </span>
          </label>
          <input
            className="modal-input"
            value={branch}
            onChange={e => setBranch(e.target.value)}
            placeholder="e.g. feature/new-feature"
          />
          <label className="modal-label">Base Branch <span style={{ color: '#6c7086', fontSize: 12 }}>(optional, default: チェックアウト中ブランチ)</span></label>
          <input
            className="modal-input"
            value={baseBranch}
            onChange={e => setBaseBranch(e.target.value)}
            placeholder="e.g. main, develop"
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
        <div className="modal-steps">
          <div className="modal-steps-label">実行コマンド</div>
          {steps.map((step, i) => (
            <div key={i} className="modal-step">{i + 1}. {step}</div>
          ))}
        </div>
      </div>
    </div>
  )
}
