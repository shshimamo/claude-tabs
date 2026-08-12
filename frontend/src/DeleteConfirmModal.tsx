import { useState } from 'react'
import { t, type Locale } from './i18n'

type Props = {
  info: {
    hasWorktree: boolean
    hasSbx: boolean
    worktreePath: string
    sbxName: string
  }
  onConfirm: (removeWorktree: boolean, removeSbx: boolean, sendExit: boolean) => void
  onCancel: () => void
  locale: Locale
}

export default function DeleteConfirmModal({ info, onConfirm, onCancel, locale }: Props) {
  const [removeWt, setRemoveWt] = useState(false)
  const [removeSbx, setRemoveSbx] = useState(false)
  const [sendExit, setSendExit] = useState(false)

  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3>{t('delete_session', locale)}</h3>
          <button className="modal-close" onClick={onCancel}>x</button>
        </div>
        <div className="modal-body">
          <p style={{ marginBottom: '12px' }}>{t('delete_resources', locale)}</p>
          {info.hasWorktree && (
            <label className="delete-check-label">
              <input type="checkbox" checked={removeWt} onChange={e => setRemoveWt(e.target.checked)} />
              <span>{t('delete_worktree', locale)} <code>{info.worktreePath}</code></span>
            </label>
          )}
          {info.hasSbx && (
            <label className="delete-check-label">
              <input type="checkbox" checked={removeSbx} onChange={e => setRemoveSbx(e.target.checked)} />
              <span>{t('delete_sandbox', locale)} <code>{info.sbxName}</code></span>
            </label>
          )}
          <label className="delete-check-label">
            <input type="checkbox" checked={sendExit} onChange={e => setSendExit(e.target.checked)} />
            <span>{t('send_exit', locale)}</span>
          </label>
        </div>
        <div className="modal-footer">
          <button className="action-btn" onClick={onCancel}>Cancel</button>
          <button className="action-btn deny-btn" onClick={() => onConfirm(removeWt, removeSbx, sendExit)}>
            {t('delete', locale)}
          </button>
        </div>
      </div>
    </div>
  )
}
