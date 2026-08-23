# claude-tabs

Claude Code の複数セッションをブラウザで一元管理するツール。

Claude Code hooks でセッション状態をリアルタイム検知し、WebSocket 経由でブラウザに反映する。

## 機能

### セッション管理
- セッション状態のリアルタイム表示（AI作業中 / 回答待ち / 許可待ち / 入力待ち / 終了）
- ステータス別グルーピング
- セッション名のカスタマイズ
- 時間ベースの非アクティブ検出（1h / 3h / 12h / 24h）
- セッションメモ（Markdown 表示、クリック編集、フォーカスアウトで自動保存）
- サイドバー幅のドラッグリサイズ（localStorage 永続化）

### プロジェクト管理
- プロジェクト作成・アーカイブ・削除
- セッションのプロジェクトへのドラッグ&ドロップ割り当て
- プロジェクト内セッションの手動並び替え（ドラッグ&ドロップ）
- プロジェクト詳細画面（リンクセクション + Markdown メモ）
- カスタマイズ可能なリンクセクション（GitHub / PRD / Spec / NotebookLM / Slack 等、追加・削除・リネーム自由）
- デフォルトリンクセクションの config 設定
- 全フィールド自動保存（フォーカスアウトで保存、ボタン操作は即時保存）
- セクション削除時の確認ダイアログ

### 会話管理
- 会話エントリのサーバーサイド保存（セッション別 JSON ファイル）
- お気に入りマーク（★ トグル、フィルタ、お気に入り以外の一括削除）
- 自動トリム時のお気に入り保護
- `conversation_max_entries` で最大保持数を設定可能

### ターミナル操作
- ターミナルフォーカス（iTerm2 / Terminal.app / カスタム対応）
- ブラウザからターミナルへの入力送信（定型文ボタン + 自由入力）
- 許可プロンプトの操作（Allow / Allow Always / Deny / Sync待ち）
- Terminal Preview（ターミナル画面の末尾を表示、idle 以外で自動リフレッシュ）
- セッション選択時のターミナル自動フォーカス（設定可）

### 通知・フォーカス
- デスクトップ通知 + 通知音（ステータス変化時、ブラウザ Notification API）
- アテンション UI（ヘッダー色変化 + サイドバーパルス + ステータス別背景色）
- 回答待ち/許可待ち時のブラウザ自動フォーカス（PWA/Chrome 対応、設定可）

### i18n
- UI 言語切り替え（英語 / 日本語）
- `locale` 設定で切り替え（デフォルト: `"en"`）

### Git Clone
- GUI から Git リポジトリをクローン
- `clone_base`（デフォルト `~/src`）にクローン

### sbx 管理
- **Create sbx**: sbx 名を入力するだけで作成（`clone_base` + デフォルトマウント自動構成、post-create コマンド自動実行）
- **Attach sbx**: 既存 sbx にアタッチして任意のリポジトリで Claude 起動（sbx 内リポジトリを動的検出）
- **Dockerfile テンプレート**: `~/.sbx/Dockerfile` の編集・保存・ビルドを GUI で完結（デフォルト雛形付き）
- Attach sbx 時のプロジェクト同時作成（GitHub / Slack リンク付き）
- セッション削除時の sbx 同時削除

### Worktree + sbx
- Worktree + sbx + Claude 自動起動（Web UI / CLI）
- セッション削除時の Worktree / sbx 同時削除
- Base Branch 指定（worktree 作成時のベースブランチ）
- PR リンクからブランチ自動解決（`gh pr view` 使用）

### 設定・表示
- AI の最終出力・ユーザー入力・許可リクエスト詳細の表示
- 会話履歴の表示（JSONL トランスクリプト読み込み）
- 定型文のカスタマイズ（`~/.claude-tabs/config.json`）
- Settings モーダル（config.json の Web UI 編集）

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
        ├── Sidebar.tsx      # セッション一覧（プロジェクト別 + ステータス別グループ）
        ├── SessionDetail.tsx # セッション詳細、入力送信、許可操作
        ├── ProjectDetail.tsx # プロジェクト詳細（リンクセクション + メモ）
        ├── WorktreeModal.tsx # Worktree作成モーダル
        ├── SbxRunModal.tsx  # 既存 sbx アタッチモーダル
        ├── CreateSbxModal.tsx # sbx 作成モーダル
        ├── CloneModal.tsx   # Git clone モーダル
        ├── DockerfileModal.tsx # Dockerfile テンプレート編集モーダル
        ├── DeleteConfirmModal.tsx # セッション削除確認（Worktree/sbx）
        ├── ConfigModal.tsx  # Settings モーダル（config.json 編集）
        ├── i18n.ts          # 多言語対応（en/ja）
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
├── conversations/           # 会話エントリ JSON（セッション別）
│   ├── {session_id}.json
│   └── ...
├── projects.json            # プロジェクト + セッション紐づけ + セッション順序
└── config.json              # 全設定（定型文 / Worktree / sbx / プロジェクト）
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
| gh | PR リンクからブランチ解決 | PR リンク使用時 |

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
    ],
    "PostToolUse": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs hook PostToolUse --claude-pid $PPID" }]
      }
    ],
    "SessionEnd": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs hook SessionEnd --claude-pid $PPID" }]
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
    ],
    "PostToolUse": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs-linux hook PostToolUse --claude-pid $PPID" }]
      }
    ],
    "SessionEnd": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "~/.claude-tabs/bin/claude-tabs-linux hook SessionEnd --claude-pid $PPID" }]
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
| `clone_base`           | Git Clone / sbx       | `git clone` 先 / sbx マウントのベースディレクトリ | `~/src` |
| `repository_base`      | Attach sbx            | Git リポジトリ検索のベースディレクトリ（`~` 展開可、深さ4まで探索） | — |
| `locale`               | 表示カスタマイズ         | UI言語（`"en"` or `"ja"`） | `"en"` |
| `statuses`             | 表示カスタマイズ         | ステータス別設定（`{ "color": "R, G, B", "opacity": 0.15, "label": "表示名" }`） | 内蔵デフォルト |
| `conversation_max_entries` | 会話管理           | 会話エントリの最大保持数（お気に入りは除外） | `100` |
| `project`              | プロジェクト管理         | プロジェクト関連設定（下記参照） | `{}` |
| `project.default_link_sections` | プロジェクト管理 | 新規プロジェクト作成時のデフォルトリンクセクション | PRD / Spec / NotebookLM / Slack |
| `focus_terminal_on_select` | フォーカス設定       | セッション選択時にターミナルを自動フォーカス | `false` |
| `focus_browser_on_attention` | フォーカス設定     | ブラウザ自動フォーカス（下記参照） | `{ "enable": false }` |
| `browser_app`          | フォーカス設定           | ブラウザのアプリ名（PWA/ショートカット用）。未設定なら Chrome でタブ検索 | — |
| `screen_lines`         | Terminal Preview      | ターミナルプレビューの表示行数 | `20` |
| `preview_interval`     | Terminal Preview      | 自動リフレッシュ間隔（秒） | `10` |
| `conversation`         | 表示カスタマイズ         | Conversation 表示設定（`{ "height": "70vh", "content_height": "200px" }`） | 内蔵デフォルト |
| `port`                 | サーバー設定           | サーバーのリッスンポート | `6277` |
| `listen_address`       | サーバー設定           | リッスンアドレス（`0.0.0.0` でLAN公開） | `localhost` |
| `terminal`             | ターミナル連携           | 使用ターミナル（`iterm2` / `terminal` / カスタム名） | `iterm2` |
| `terminal_presets`     | ターミナル連携           | ターミナル操作の AppleScript 定義（カスタムターミナル対応用） | 内蔵(iterm2, terminal) |

### ステータス一覧

| ステータス | 説明 |
|-----------|------|
| `ai_working` | AI 作業中 |
| `waiting_input` | 回答待ち（AI が質問を出している） |
| `permission_required` | 許可待ち（ツール実行の承認待ち） |
| `idle` | 入力待ち（ユーザーのプロンプト待ち） |
| `inactive_1h` | 1時間以上非アクティブ |
| `inactive_3h` | 3時間以上非アクティブ |
| `inactive_12h` | 12時間以上非アクティブ |
| `inactive_24h` | 24時間以上非アクティブ |
| `terminated` | 終了済み |

### ブラウザ自動フォーカス

`focus_browser_on_attention` でステータス変化時にブラウザを自動で前面に表示。

```json
{
  "focus_browser_on_attention": {
    "enable": true,
    "statuses": ["waiting_input", "permission_required"]
  }
}
```

- `enable`: 有効/無効
- `statuses`: フォーカス対象のステータス（上記ステータス一覧から指定）。省略時は `["waiting_input", "permission_required"]`
- `browser_app` でPWA名を指定可能。未設定なら Chrome でタブ検索

### プロジェクト設定

```json
{
  "project": {
    "default_link_sections": [
      { "label": "PRD" },
      { "label": "Spec" },
      { "label": "Asana" },
      { "label": "Slack" }
    ]
  }
}
```

未設定時のデフォルト: GitHub / PRD / Spec / NotebookLM / Slack。プロジェクト作成後は各プロジェクトで自由にセクションの追加・削除・リネームが可能。

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

Web UI の「+ Worktree」ボタン(または CLI) から、worktree 作成 + sbx セットアップ + Claude 自動起動が可能。

#### sbx 側の必須セットアップ

1. Linux 用 hooks バイナリをビルド・配置（`make install-hook-linux`）
2. sbx 内の `~/.claude/settings.json` に Linux 用 hooks 設定を追加
   - hooks 設定は `sbx_post_create_cmds` に含めることで自動化できる（参考: [`examples/sbx-setup.sh`](examples/sbx-setup.sh)）。

`~/.claude-tabs` のマウントとシンボリックリンク作成はコードで自動実行されるため手動設定は不要。

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
