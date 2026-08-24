import { useState, useEffect, useMemo } from 'react'

type SbxConfig = {
  template?: string
  default_mounts?: string[]
  kits?: string[]
  post_create_cmds?: string[][]
  plugins?: { source: string; plugins: string[] }[]
  clone_base?: string
}

type Config = {
  sbx?: SbxConfig
  worktree?: { base?: string }
}

type Props = {
  onClose: () => void
}

export default function ManageSbxModal({ onClose }: Props) {
  const [sbxList, setSbxList] = useState<string[]>([])
  const [name, setName] = useState('')
  const [creating, setCreating] = useState(false)
  const [deleting, setDeleting] = useState<string | null>(null)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)
  const [cfg, setCfg] = useState<Config>({})

  const refresh = () => {
    fetch('/api/sbx/list').then(r => r.json()).then(setSbxList).catch(() => {})
  }

  useEffect(() => {
    refresh()
    fetch('/api/config').then(r => r.json()).then(setCfg).catch(() => {})
  }, [])

  const handleCreate = async () => {
    if (!name.trim()) return
    setCreating(true)
    setResult(null)
    try {
      const res = await fetch(`/api/sbx/create?name=${encodeURIComponent(name.trim())}`, { method: 'POST' })
      const data = await res.json()
      setResult({ ok: res.ok, message: data.message })
      if (res.ok) { setName(''); refresh() }
    } catch {
      setResult({ ok: false, message: 'Request failed' })
    }
    setCreating(false)
  }

  const handleDelete = async (sbxName: string) => {
    if (!confirm(`Delete "${sbxName}"?`)) return
    setDeleting(sbxName)
    setResult(null)
    try {
      const res = await fetch(`/api/sbx/delete?name=${encodeURIComponent(sbxName)}`, { method: 'POST' })
      const data = await res.json()
      setResult({ ok: res.ok, message: data.message })
      if (res.ok) refresh()
    } catch {
      setResult({ ok: false, message: 'Request failed' })
    }
    setDeleting(null)
  }

  const sbx = cfg.sbx || {}
  const template = sbx.template || 'my-sbx:latest'
  const cloneBase = sbx.clone_base || '~/src'
  const n = name.trim() || '<name>'

  const steps = useMemo(() => {
    const s: string[] = []
    const wtBase = cfg.worktree?.base
    const mounts = [cloneBase, ...(wtBase ? [wtBase] : []), '~/.claude-tabs', ...(sbx.default_mounts || [])]
    const kits = (sbx.kits || []).map(k => `--kit ${k}`).join(' ')
    s.push(`sbx create --name ${n} -t ${template}${kits ? ' ' + kits : ''} claude ${mounts.join(' ')}`)
    s.push(`sbx exec ${n} ln -sf ~/.claude-tabs ~/.claude-tabs`)
    for (const cmd of sbx.post_create_cmds || []) {
      s.push(`sbx exec ${n} ${cmd.join(' ')}`)
    }
    for (const pc of sbx.plugins || []) {
      s.push(`sbx exec ${n} claude plugins marketplace add ${pc.source}`)
      for (const p of pc.plugins) {
        if (p === 'auto') {
          s.push(`sbx exec ${n} claude plugins install <auto-detected>`)
        } else {
          s.push(`sbx exec ${n} claude plugins install ${p}`)
        }
      }
    }
    return s
  }, [n, template, cloneBase, sbx])

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal wt-modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Manage sbx</h3>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="modal-body">
          <label className="modal-label">Sandboxes</label>
          {sbxList.length > 0 ? (
            <div className="checkout-repo-list">
              {sbxList.map(s => (
                <div key={s} className="checkout-repo-item" style={{ justifyContent: 'space-between' }}>
                  <span>{s}</span>
                  <button
                    className="action-btn"
                    style={{ padding: '2px 8px', fontSize: '0.8em' }}
                    onClick={() => handleDelete(s)}
                    disabled={deleting === s}
                  >
                    {deleting === s ? '...' : 'Delete'}
                  </button>
                </div>
              ))}
            </div>
          ) : (
            <div style={{ color: '#6c7086', fontSize: '0.9em', padding: '8px 0' }}>No sandboxes</div>
          )}

          <label className="modal-label" style={{ marginTop: 16 }}>Create New</label>
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              className="modal-input"
              style={{ flex: 1 }}
              value={name}
              onChange={e => setName(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter' && name.trim()) handleCreate() }}
              placeholder="e.g. my-review"
            />
            <button
              className="action-btn allow-btn"
              onClick={handleCreate}
              disabled={creating || !name.trim()}
            >
              {creating ? '...' : 'Create'}
            </button>
          </div>
          {name.trim() && (
            <div className="modal-steps" style={{ marginTop: 8 }}>
              <div className="modal-steps-label">Commands</div>
              {steps.map((step, i) => (
                <div key={i} className="modal-step">{step}</div>
              ))}
            </div>
          )}

          {result && (
            <div className={`modal-result ${result.ok ? 'modal-result-ok' : 'modal-result-err'}`}>
              {result.message}
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button className="action-btn" onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  )
}
