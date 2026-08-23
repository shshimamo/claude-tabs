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
}

type Props = {
  onClose: () => void
}

export default function CreateSbxModal({ onClose }: Props) {
  const [name, setName] = useState('')
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)
  const [cfg, setCfg] = useState<Config>({})

  useEffect(() => {
    fetch('/api/config').then(r => r.json()).then(setCfg).catch(() => {})
  }, [])

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

  const sbx = cfg.sbx || {}
  const template = sbx.template || 'my-sbx:latest'
  const cloneBase = sbx.clone_base || '~/src'
  const n = name.trim() || '<name>'

  const steps = useMemo(() => {
    const s: string[] = []
    const mounts = [cloneBase, '~/.claude-tabs', ...(sbx.default_mounts || [])]
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
          {name.trim() && (
            <div className="modal-steps">
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
          <button className="action-btn" onClick={onClose} disabled={running}>Cancel</button>
          <button className="action-btn allow-btn" onClick={handleCreate} disabled={running || !name.trim()}>
            {running ? 'Creating...' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  )
}
