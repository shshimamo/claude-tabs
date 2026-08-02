import { useState } from 'react'

type Props = {
  info: {
    hasWorktree: boolean
    hasSbx: boolean
    worktreePath: string
    sbxName: string
  }
  onConfirm: (removeWorktree: boolean, removeSbx: boolean) => void
  onCancel: () => void
}

export default function DeleteConfirmModal({ info, onConfirm, onCancel }: Props) {
  const [removeWt, setRemoveWt] = useState(false)
  const [removeSbx, setRemoveSbx] = useState(false)

  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3>セッション削除</h3>
          <button className="modal-close" onClick={onCancel}>x</button>
        </div>
        <div className="modal-body">
          <p style={{ marginBottom: '12px' }}>関連リソースも削除しますか？</p>
          {info.hasWorktree && (
            <label className="delete-check-label">
              <input type="checkbox" checked={removeWt} onChange={e => setRemoveWt(e.target.checked)} />
              <span>Worktree を削除 <code>{info.worktreePath}</code></span>
            </label>
          )}
          {info.hasSbx && (
            <label className="delete-check-label">
              <input type="checkbox" checked={removeSbx} onChange={e => setRemoveSbx(e.target.checked)} />
              <span>Sandbox を削除 <code>{info.sbxName}</code></span>
            </label>
          )}
        </div>
        <div className="modal-footer">
          <button className="action-btn" onClick={onCancel}>Cancel</button>
          <button className="action-btn deny-btn" onClick={() => onConfirm(removeWt, removeSbx)}>
            削除
          </button>
        </div>
      </div>
    </div>
  )
}
