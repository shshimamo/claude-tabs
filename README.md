# claude-tabs

Claude Code の複数セッションをブラウザで一元管理するツール。

Claude Code hooks でセッション状態をリアルタイム検知し、WebSocket 経由でブラウザに反映する。

## 機能

- セッション状態のリアルタイム表示（AI作業中 / 回答待ち / 許可待ち / 入力待ち / 終了）
- ステータス別グルーピング
- セッション名のカスタマイズ
- ターミナルフォーカス（iTerm2 / Terminal.app / カスタム対応）
- ブラウザからターミナルへの入力送信（定型文ボタン + 自由入力）
- 許可プロンプトの操作（Allow / Allow Always / Deny）
- AI の最終出力・ユーザー入力・許可リクエスト詳細の表示
- 会話履歴の表示（JSONL トランスクリプト読み込み）
- 時間ベースの非アクティブ検出（1h / 3h / 12h / 24h）
- デスクトップ通知 + 通知音（ステータス変化時、ブラウザ Notification API）
- アテンション UI（ヘッダー色変化 + サイドバーパルス）
- 定型文のカスタマイズ（`~/.claude-tabs/config.json`）
- Settings モーダル（config.json の Web UI 編集）
- Worktree + sbx + Claude 自動起動（Web UI / CLI）
- セッション削除時の Worktree / sbx 同時削除

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
        ├── DeleteConfirmModal.tsx # セッション削除確認（Worktree/sbx）
        ├── ConfigModal.tsx  # Settings モーダル（config.json 編集）
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

## 前提ツール

| ツール | 用途 | 必須 |
|--------|------|------|
| Go | ビルド | ○ |
| Node.js + pnpm | フロントエンドビルド | ○ |
| Claude Code | セッション管理対象 | ○ |
| macOS + osascript | ターミナル操作（フォーカス・入力送信） | ○ |
| git | Worktree 作成・削除 | Worktree 機能使用時 |
| sbx | サンドボックス環境 | Worktree + sbx 連携時 |

## 起動手順

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

## config.json について

`~/.claude-tabs/config.json` で行う（参考: [`examples/config.json`](examples/config.json)）:

| キー                     | カテゴリ              |  説明                                                      | デフォルト |
|------------------------|-------------------|----------------------------------------------------------|-------|
| `presets`              | 定型文カスタマイズ | ブラウザ UI の定型文ボタンをカスタマイズ | Yes / Commit / Commit & Push |
| `worktree_base`        | Worktree + sbx 連携 | worktree 保存先 | `~/worktrees` |
| `sbx_template`         | Worktree + sbx 連携 | sbx テンプレート | `my-sbx:latest` |
| `sbx_default_mounts`   | Worktree + sbx 連携 | sbx デフォルトマウント（`~` 展開可） | `[]` |
| `sbx_post_create_cmds` | Worktree + sbx 連携 | sbx 作成後に順次実行するコマンド群（`[["cmd", "arg"], ...]` 形式） | `[]` |
| `sbx_kits`             | Worktree + sbx 連携 | sbx 作成時に適用する kit（ディレクトリ / ZIP / OCI） | `[]` |
| `plugins`              | Worktree + sbx 連携 | プラグイン設定の配列 | `[]` |
| `plugins[].source`     | Worktree + sbx 連携 | ローカルパス（`~` 展開可）または GitHub URL（`user/repo`、`https://...`） | — |
| `plugins[].plugins`    | Worktree + sbx 連携 | インストールするプラグイン名。`["auto"]` でローカルの `plugins/` から自動検出 | — |
| `terminal`             | ターミナル連携           | 使用ターミナル（`iterm2` / `terminal` / カスタム名） | `iterm2` |
| `terminal_presets`     | ターミナル連携           | ターミナル操作の AppleScript 定義（カスタムターミナル対応用） | 内蔵(iterm2, terminal) |

### 定型文カスタマイズ

config.json 表の `定型文カスタマイズ` で設定。

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

### Worktree + sbx 連携

config.json 表の `Worktree + sbx 連携` で設定。

Web UI の「+ New Worktree」ボタン(または CLI) から、worktree 作成 + sbx セットアップ + Claude 自動起動が可能。

### ターミナル設定

config.json 表の `ターミナル設定` で設定。

`iterm2` と `terminal`（Terminal.app）は内蔵プリセットがあり、設定不要で動作する。

```json
{ "terminal": "iterm2" }
```

`terminal_presets` に同名のエントリを書けば内蔵より優先される。
他のターミナル（Ghostty 等）を使う場合は `terminal_presets` に AppleScript を定義して `terminal` で名前を指定する。

テンプレート変数: `{{TTY}}`, `{{TEXT}}`, `{{CMDS}}`, `{{COMMAND}}`

詳細は [`examples/config.json`](examples/config.json) を参照。

## CLI モード

```sh
claude-tabs                                  # ブラウザを開く（サーバーが未起動なら自動起動）
claude-tabs --server                         # サーバーモードで直接起動
claude-tabs hook <EventType>                 # hook ハンドラー（Claude Code から呼ばれる）
claude-tabs worktree create <repo> <branch>  # worktree + sbx + Claude 起動
```

### zsh 関数

ターミナルから使う場合の参考例: [`examples/zsh-functions.sh`](examples/zsh-functions.sh)

使用ツール: `fzf`（wtcd / wtrm）、`sbx`（wt-sbx / wtrm）

```sh
wt-sbx <repo> <branch>  # claude-tabs worktree create のラッパー
wtcd                     # worktree一覧からfzfで選択してcd
wtrm                     # worktree + sbx を削除
```

## 技術スタック

- **Backend**: Go, fsnotify, gorilla/websocket, embed
- **Frontend**: React 18, Vite, TypeScript
- **Theme**: Catppuccin Mocha
