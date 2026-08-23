# Mac + sbx 環境セットアップ

macOS 上で sbx 内の Claude Code を管理するためのセットアップ手順。
Git Clone / Create sbx / Attach sbx 機能を使い、ターミナル操作を最小限に抑えて運用できる。

## 前提

- macOS（osascript によるターミナル操作に使用）
- Docker インストール済み
- sbx CLI インストール済み
- iTerm2 推奨（Terminal.app も可）

## 1. 前提ツールのインストール

```sh
# mise のインストール（未導入の場合）
# https://mise.jdx.dev/getting-started.html

# clone_base にリポジトリをクローン
mkdir -p ~/src
git clone https://github.com/shshimamo/claude-tabs.git ~/src/claude-tabs
cd ~/src/claude-tabs

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

## 3. config.json の設定

```sh
mkdir -p ~/.claude-tabs
cp examples/config-sbx.json ~/.claude-tabs/config.json
```

最低限の設定が入った状態で使える。必要に応じて `~/.claude-tabs/config.json` を編集。

| 設定 | 説明 | デフォルト |
|------|------|-----------|
| `sbx.clone_base` | Git Clone 先。Create sbx 時に自動マウントされる | `~/src` |
| `sbx.template` | Create sbx で使用するテンプレート名 | `my-sbx:latest` |
| `sbx.post_create_cmds` | sbx 作成後に実行するコマンド | `sbx-setup.sh`（hooks 自動設定） |

`clone_base`（`~/src`）は sbx に自動マウントされるため、`~/src/claude-tabs/examples/sbx-setup.sh` が sbx 内から参照可能。sbx 作成時に hooks（Linux 用）が自動設定される。

## 4. sbx テンプレートのビルド（初回のみ）

1. ブラウザで claude-tabs を開く（`http://localhost:6277`）
2. ヘッダーの **Dockerfile** ボタンをクリック
3. デフォルトの雛形が表示される。そのままでも動作するが、必要に応じてカスタマイズ可能
4. **Build Template** ボタンをクリック

内部で以下が実行される:
```
docker build -t my-sbx:latest -f ~/.sbx/Dockerfile ~/.sbx/
docker save my-sbx:latest -o <tmpfile>
sbx template load <tmpfile>
```

テンプレートを変更したい場合は Dockerfile を編集して再度 Build するだけ。

## 5. 使い方

### 基本的な流れ

1. **Clone**: ヘッダーの「Clone」ボタンから Git リポジトリをクローン
2. **Create sbx**: 「+ Create sbx」ボタンで sbx を作成（clone_base + hooks が自動セットアップされる）
3. **Attach sbx**: 「Attach sbx」ボタンで既存 sbx にアタッチし、リポジトリを選択して Claude を起動

以降は Web UI 上でセッションの状態確認、定型文送信、許可操作などを行える。
