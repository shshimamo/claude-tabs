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
- [Mac + sbx 環境セットアップガイド](docs/mac-sbx-setup.md)

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
| Claude Code | セッション管理対象 | ○（sbx 内で Claude Code を使う場合は不要） |
| macOS + osascript | ターミナル操作（フォーカス・入力送信） | ○ |
| git | Worktree 作成・削除、Git Clone | Worktree / Clone 機能使用時 |
| sbx | サンドボックス環境 | sbx 機能使用時（Create / Attach / Worktree+sbx） |
| gh | PR リンクからブランチ解決 | PR リンク使用時 |

## セットアップ

### 前提ツールのインストール（mise）

[mise](https://mise.jdx.dev/) を使って前提ツールを一括インストールできる（sbx は別途インストール）。

```sh
mise install
```

mise 未導入の場合: https://mise.jdx.dev/getting-started.html

### ビルド & インストール

```sh
cd frontend && pnpm install && pnpm approve-builds && cd ..
make install
```

### Claude Code hooks 設定

詳細は [docs/hooks-setup.md](docs/hooks-setup.md) を参照。

### 起動

```sh
~/.claude-tabs/bin/claude-tabs
```

ブラウザが自動で開く。サーバーが既に起動中なら既存サーバーに接続。

## config.json について

詳細は [docs/config.md](docs/config.md) を参照（参考: [`examples/config.json`](examples/config.json)）。

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
