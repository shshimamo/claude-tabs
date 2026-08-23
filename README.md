# claude-tabs

Claude Code の複数セッションをブラウザで一元管理するツール。

Claude Code hooks でセッション状態をリアルタイム検知し、WebSocket 経由でブラウザに反映する。

## 機能

| カテゴリ | 機能 |
|---------|------|
| セッション管理 | リアルタイム状態表示（AI作業中 / 回答待ち / 許可待ち / 入力待ち / 終了） |
| | ステータス別グルーピング、セッション名カスタマイズ |
| | 時間ベースの非アクティブ検出（1h / 3h / 12h / 24h） |
| | セッションメモ（Markdown 表示、クリック編集、自動保存） |
| プロジェクト管理 | プロジェクト作成・アーカイブ・削除、D&D でセッション割り当て・並び替え |
| | リンクセクション（GitHub / PRD / Spec 等、追加・削除・リネーム自由） |
| | Markdown メモ、全フィールド自動保存 |
| 会話管理 | 会話エントリの保存、お気に入りマーク・フィルタ、自動トリム |
| ターミナル操作 | フォーカス切り替え（iTerm2 / Terminal.app / カスタム） |
| | 定型文・自由入力の送信、許可プロンプト操作（Allow / Deny 等） |
| | Terminal Preview（ターミナル画面の末尾表示、自動リフレッシュ） |
| 通知・フォーカス | デスクトップ通知 + 通知音、アテンション UI |
| | 回答待ち/許可待ち時のブラウザ自動フォーカス（PWA 対応） |
| Git Clone | GUI からリポジトリをクローン（`sbx.clone_base` に保存） |
| sbx 管理 | Create sbx / Attach sbx / Dockerfile テンプレート編集・ビルド |
| | Attach 時のプロジェクト同時作成、削除時の sbx 同時削除 |
| | [Mac + sbx 環境セットアップガイド](docs/mac-sbx-setup.md) |
| Worktree + sbx | Worktree + sbx + Claude 自動起動（Web UI / CLI） |
| | Base Branch 指定、PR リンクからブランチ自動解決 |
| 設定・表示 | 会話履歴表示、定型文カスタマイズ、Settings モーダル |
| i18n | UI 言語切り替え（英語 / 日本語） |

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

### ビルド & 起動

```sh
# 初回のみ
cd frontend && pnpm install && pnpm approve-builds && cd ..

# ビルド + インストール + 起動（2回目以降はこれだけ）
make restart
```

ブラウザで `http://{listen_address}:{port}`（デフォルト: http://localhost:6277）が自動で開く。サーバーが既に起動中なら既存サーバーに接続。

### Claude Code hooks 設定

詳細は [docs/hooks-setup.md](docs/hooks-setup.md) を参照。

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
