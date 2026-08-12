export type Locale = 'en' | 'ja'

const messages = {
  // Status labels
  status_ai_working:          { en: 'Working',      ja: 'AI作業中' },
  status_waiting_input:       { en: 'Waiting',      ja: '回答待ち' },
  status_permission_required: { en: 'Permission',   ja: '許可待ち' },
  status_idle:                { en: 'Idle',          ja: '入力待ち' },
  status_inactive_1h:         { en: 'Inactive 1h',  ja: '1時間経過' },
  status_inactive_3h:         { en: 'Inactive 3h',  ja: '3時間経過' },
  status_inactive_12h:        { en: 'Inactive 12h', ja: '12時間経過' },
  status_inactive_24h:        { en: 'Inactive 24h', ja: '24時間経過' },
  status_terminated:          { en: 'Terminated',    ja: '終了' },

  // Notification
  notify_idle:                { en: 'Idle',                ja: '入力待ち' },
  notify_waiting_input:       { en: 'Waiting for input',   ja: '回答待ち' },
  notify_permission_required: { en: 'Permission required', ja: '許可待ち' },

  // Sidebar
  no_sessions:     { en: 'No sessions', ja: 'セッションなし' },

  // SessionDetail
  send_choice:     { en: 'Send choice',        ja: '選択肢を送信' },
  sent:            { en: 'Sent ✓',             ja: '送信済み ✓' },
  loading:         { en: 'Loading...',          ja: '読み込み中...' },
  no_content:      { en: 'No content',          ja: '内容なし' },
  presets:         { en: 'Presets',             ja: '定型文' },
  free_input:      { en: 'Free input',          ja: '自由入力' },
  type_to_send:    { en: 'Type to send...',     ja: '入力を送信...' },
  send_shortcut:   { en: 'Send (⌘+Enter)',     ja: '送信 (⌘+Enter)' },
  enter_memo:      { en: 'Enter memo...',       ja: 'メモを入力...' },
  click_to_set_tty:{ en: 'Click to set TTY',    ja: 'TTYを設定' },
  tty_placeholder: { en: '/dev/ttys001 (for Focus Terminal in remote env)', ja: '/dev/ttys001 (sbx等リモート環境のFocus Terminal用)' },
  no_history:      { en: 'No conversation history', ja: '会話履歴が見つからない' },

  // App
  empty_hint:      { en: 'No sessions. Set up Claude Code hooks to get started.', ja: 'セッションなし。Claude Code hooks を設定してセッションを開始してください。' },

  // DeleteConfirmModal
  delete_session:  { en: 'Delete Session',      ja: 'セッション削除' },
  delete_resources:{ en: 'Also delete related resources?', ja: '関連リソースも削除しますか？' },
  delete_worktree: { en: 'Delete worktree',      ja: 'Worktree を削除' },
  delete_sandbox:  { en: 'Delete sandbox',       ja: 'Sandbox を削除' },
  send_exit:       { en: 'Send /exit to terminate Claude Code', ja: '/exit を送信して Claude Code を終了' },
  delete:          { en: 'Delete',               ja: '削除' },

  // WorktreeModal / SbxRunModal tooltips
  branch_tooltip:  {
    en: '1. Worktree already exists → use as-is\n2. origin/branch exists → create from remote branch\n3. Local branch exists → create from local branch\n4. Neither exists → create new from base branch (default: current checked-out branch)',
    ja: '1. worktree が既に存在 → そのまま使用\n2. origin/branch が存在 → リモートブランチから作成\n3. ローカルに branch が存在 → ローカルブランチから作成\n4. どちらもない → base branch (default: リポジトリのチェックアウト中ブランチ) から新規作成',
  },
  base_branch_hint:{ en: '(optional, default: current branch)', ja: '(optional, default: チェックアウト中ブランチ)' },
  commands:        { en: 'Commands',             ja: '実行コマンド' },
} as const

export type MessageKey = keyof typeof messages

export function t(key: MessageKey, locale: Locale): string {
  return messages[key][locale]
}

export function statusLabel(status: string, locale: Locale, customLabels?: Record<string, string>): string {
  if (customLabels?.[status]) return customLabels[status]
  const key = `status_${status}` as MessageKey
  if (key in messages) return messages[key][locale]
  return status
}

export function notifyLabel(status: string, locale: Locale, customLabels?: Record<string, string>): string {
  if (customLabels?.[status]) return customLabels[status]
  const key = `notify_${status}` as MessageKey
  if (key in messages) return messages[key][locale]
  return status
}
