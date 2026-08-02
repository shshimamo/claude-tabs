# claude-tabs

Claude Code の複数セッションをブラウザで一元管理するツール。

Claude Code hooks でセッション状態をリアルタイム検知し、WebSocket 経由でブラウザに反映する。

## 機能

- セッション状態のリアルタイム表示（AI作業中 / 回答待ち / 許可待ち / 入力待ち / 終了）
- ステータス別グルーピング
- セッション名のカスタマイズ
- iTerm2 ターミナルフォーカス（AppleScript）
- ブラウザからターミナルへの入力送信（定型文ボタン + 自由入力）
- 許可プロンプトの操作（Allow / Allow Always / Deny）
- AI の最終出力・ユーザー入力・許可リクエスト詳細の表示
- 会話履歴の表示（JSONL トランスクリプト読み込み）
- 時間ベースの非アクティブ検出（1h / 3h / 12h / 24h）
- デスクトップ通知 + 通知音（ステータス変化時、ブラウザ Notification API）
- アテンション UI（ヘッダー色変化 + サイドバーパルス）
- 定型文のカスタマイズ（`~/.claude-tabs/config.json`）
- Worktree + sbx + Claude 自動起動（Web UI / CLI）

## アーキテクチャ

```
[Claude Code]
  ── hook ──> ~/.claude-tabs/sessions/{session_id}.json
                ↓ (fsnotify)
          [Go Server (localhost:6277)]
            ├─ REST API
            ├─ WebSocket (リアルタイム更新)
            └─ embed (React frontend)
                ↓
          [Browser UI]
```

## ディレクトリ構成

```
claude-tabs/
├── main.go                  # Go サーバー / hook ハンドラー / クライアント
├── go.mod
├── go.sum
├── Makefile
├── hooks-setup.json         # Claude Code hooks 設定の見本
├── .gitignore
└── frontend/                # React + Vite + TypeScript
    ├── index.html
    ├── package.json
    ├── tsconfig.json
    ├── vite.config.ts
    └── src/
        ├── main.tsx         # エントリーポイント
        ├── App.tsx          # メインレイアウト、WebSocket 接続
        ├── Sidebar.tsx      # セッション一覧（ステータス別グループ）
        ├── SessionDetail.tsx # セッション詳細、入力送信、許可操作
        ├── WorktreeModal.tsx # Worktree作成モーダル
        └── index.css        # Catppuccin ダークテーマ
```

### ランタイムディレクトリ

```
~/.claude-tabs/
├── bin/
│   └── claude-tabs          # インストール先バイナリ
├── sessions/                # セッション状態 JSON（hook が書き込み）
│   ├── {session_id}.json
│   └── ...
└── config.json              # 全設定（定型文 / Worktree / sbx / プラグイン）
```

## セットアップ

### 1. ビルド & インストール

```sh
cd frontend && pnpm install && pnpm approve-builds && cd ..
make install
```

### 2. Claude Code hooks 設定

`~/.claude/settings.json` の `hooks` セクションに以下を追加（`hooks-setup.json` を参照）:

**macOS（hook 実行環境が macOS の場合）:**

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs hook SessionStart --claude-pid $PPID" }]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs hook UserPromptSubmit --claude-pid $PPID" }]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "AskUserQuestion",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs hook AskUserQuestion --claude-pid $PPID" }]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs hook Stop --claude-pid $PPID" }]
      }
    ],
    "PermissionRequest": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs hook PermissionRequest --claude-pid $PPID" }]
      }
    ]
  }
}
```

**Linux（sbx など hook 実行環境が Linux の場合）:**

`make install-hook-linux` で Linux 用バイナリをビルドし、`claude-tabs-linux` を使用する（`hooks-setup-linux.json` を参照）:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs-linux hook SessionStart --claude-pid $PPID" }]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs-linux hook UserPromptSubmit --claude-pid $PPID" }]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "AskUserQuestion",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs-linux hook AskUserQuestion --claude-pid $PPID" }]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs-linux hook Stop --claude-pid $PPID" }]
      }
    ],
    "PermissionRequest": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs-linux hook PermissionRequest --claude-pid $PPID" }]
      }
    ]
  }
}
```

### 3. 起動

```sh
~/.claude-tabs/bin/claude-tabs
```

ブラウザが自動で開く。サーバーが既に起動中なら既存サーバーに接続。

## 定型文カスタマイズ

`~/.claude-tabs/config.json` の `presets` でブラウザ UI の定型文ボタンをカスタマイズできる:

```json
{
  "presets": [
    { "label": "Yes", "text": "yes" },
    { "label": "Commit", "text": "commit して" },
    { "label": "Commit & Push", "text": "commit して push して" }
  ]
}
```

未設定の場合は上記デフォルトが使用される。

## Worktree + sbx 連携

Web UI の「+ New Worktree」ボタン(または CLI) から、worktree 作成 + sbx セットアップ + Claude 自動起動が可能。

### 設定

Worktree + sbx の設定も `~/.claude-tabs/config.json` で行う（参考: [`examples/config.json`](examples/config.json)）:

| キー | 説明 |
|------|------|
| `worktree_base` | worktree 保存先（空なら `$(ghq root)/worktrees`） |
| `sbx_template` | sbx テンプレート（デフォルト: `my-sbx:latest`） |
| `sbx_default_mounts` | sbx デフォルトマウント（`~` 展開可） |
| `sbx_setup_cmd` | sbx 作成後に実行するセットアップコマンド（参考: [`examples/sbx-setup.sh`](examples/sbx-setup.sh)） |
| `plugins` | プラグイン設定の配列 |
| `plugins[].source` | ローカルパス（`~` 展開可）または GitHub URL（`user/repo`、`https://...`） |
| `plugins[].plugins` | インストールするプラグイン名。`["auto"]` でローカルの `plugins/` から自動検出 |

## CLI モード

```sh
claude-tabs                                  # ブラウザを開く（サーバーが未起動なら自動起動）
claude-tabs --server                         # サーバーモードで直接起動
claude-tabs hook <EventType>                 # hook ハンドラー（Claude Code から呼ばれる）
claude-tabs worktree create <repo> <branch>  # worktree + sbx + Claude 起動
```

### zsh 関数

ターミナルから使う場合の参考例: [`examples/zsh-functions.sh`](examples/zsh-functions.sh)

```sh
wt-sbx <repo> <branch>  # claude-tabs worktree create のラッパー
wtcd                     # worktree一覧からfzfで選択してcd
wtrm                     # worktree + sbx を削除
```

## 技術スタック

- **Backend**: Go, fsnotify, gorilla/websocket, embed
- **Frontend**: React 18, Vite, TypeScript
- **Theme**: Catppuccin Mocha
