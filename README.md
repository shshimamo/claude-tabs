# claude-tabs

Claude Code の複数セッションをブラウザで一元管理するツール。

Claude Code hooks でセッション状態をリアルタイム検知し、WebSocket 経由でブラウザに反映する。

## 機能

- セッション状態のリアルタイム表示（AI Working / Waiting Input / Permission Required / Idle / Terminated）
- ステータス別グルーピング
- セッション名のカスタマイズ
- iTerm2 ターミナルフォーカス（AppleScript）
- 会話履歴の表示（JSONL トランスクリプト読み込み）
- プロセス生存チェックによる自動 Terminated 検出

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
        ├── SessionDetail.tsx # セッション詳細、名前編集、履歴表示
        └── index.css        # Catppuccin ダークテーマ
```

### ランタイムディレクトリ

```
~/.claude-tabs/
├── bin/
│   └── claude-tabs          # インストール先バイナリ
└── sessions/                # セッション状態 JSON（hook が書き込み）
    ├── {session_id}.json
    └── ...
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

`make install-hook-linux` で Linux 用バイナリをビルドし、`claude-tabs-linux` を使用する:

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

## CLI モード

```sh
claude-tabs                    # ブラウザを開く（サーバーが未起動なら自動起動）
claude-tabs --server           # サーバーモードで直接起動
claude-tabs hook <EventType>   # hook ハンドラー（Claude Code から呼ばれる）
```

## 技術スタック

- **Backend**: Go, fsnotify, gorilla/websocket, embed
- **Frontend**: React 18, Vite, TypeScript
- **Theme**: Catppuccin Mocha
