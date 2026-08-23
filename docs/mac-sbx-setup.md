# Mac + sbx 環境セットアップ

macOS 上で sbx 内の Claude Code を管理するためのセットアップ手順。
Git Clone / Create sbx / Attach sbx 機能を使い、ターミナル操作を最小限に抑えて運用できる。

## 前提

- macOS（osascript によるターミナル操作に使用）
- sbx CLI インストール済み
- iTerm2 推奨（Terminal.app も可）

## 1. 前提ツールのインストール

```sh
# mise のインストール（未導入の場合）
# https://mise.jdx.dev/getting-started.html

# リポジトリをクローン
git clone https://github.com/shshimamo/claude-tabs.git
cd claude-tabs

# 前提ツール（Go, Node.js, pnpm, gh）を一括インストール
mise install
```

## 2. ビルド & 起動

```sh
# 初回のみ
cd frontend && pnpm install && pnpm approve-builds && cd ..

# ビルド + インストール + 起動（2回目以降はこれだけ）
make restart

# sbx 内の Claude Code hooks 用 Linux バイナリ生成
make install-hook-linux
```

## 3. Claude Code hooks 設定

sbx 内で Claude Code を使う場合、hooks は Linux 用バイナリ（`claude-tabs-linux`）を指定する。

`~/.claude/settings.json` の `hooks` セクションに以下を追加（`hooks-setup-linux.json` を参照）:

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

## 4. config.json の設定

`~/.claude-tabs/config.json` を作成し、sbx 関連の設定を行う。

```json
{
  "sbx": {
    "clone_base": "~/src",
    "template": "my-sbx:latest",
    "default_mounts": [
      "~/dotfiles:ro"
    ],
    "post_create_cmds": []
  },
  "terminal": "iterm2"
}
```

| 設定 | 説明 |
|------|------|
| `sbx.clone_base` | Git Clone 先のベースディレクトリ。Create sbx 時に自動マウントされる |
| `sbx.template` | Create sbx で使用するテンプレート名 |
| `sbx.default_mounts` | sbx 作成時にマウントするディレクトリ（`clone_base` は自動追加） |
| `sbx.post_create_cmds` | sbx 作成後に実行するコマンド |

## 5. Dockerfile テンプレートの準備（任意）

Web UI の「Dockerfile」ボタンからテンプレートの編集・ビルドが可能。
デフォルトの雛形が用意されているので、そのままビルドするか、必要に応じてカスタマイズする。

## 6. 起動 & 使い方

```sh
claude-tabs
```

ブラウザで `http://localhost:6277` を開く。

### 基本的な流れ

1. **Clone**: ヘッダーの「Clone」ボタンから Git リポジトリをクローン
2. **Create sbx**: 「+ Create sbx」ボタンで sbx を作成（clone_base が自動マウントされる）
3. **Attach sbx**: 「Attach sbx」ボタンで既存 sbx にアタッチし、リポジトリを選択して Claude を起動

以降は Web UI 上でセッションの状態確認、定型文送信、許可操作などを行える。
