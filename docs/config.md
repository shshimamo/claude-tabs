# config.json リファレンス

`~/.claude-tabs/config.json` で設定（参考: [`../examples/config.json`](../examples/config.json)）。

## 設定一覧

| キー                     | カテゴリ              |  説明                                                      | デフォルト |
|------------------------|-------------------|----------------------------------------------------------|-------|
| `presets`              | 定型文カスタマイズ | ブラウザ UI の定型文ボタンをカスタマイズ | Yes / Commit / Commit & Push |
| `worktree.base`            | Worktree | worktree 保存先 | `~/worktrees` |
| `sbx.template`             | sbx      | sbx テンプレート | `my-sbx:latest` |
| `sbx.default_mounts`       | sbx      | sbx デフォルトマウント（`~` 展開可） | `[]` |
| `sbx.post_create_cmds`     | sbx      | sbx 作成後に順次実行するコマンド群（`[["cmd", "arg"], ...]` 形式） | `[]` |
| `sbx.kits`                 | sbx      | sbx 作成時に適用する kit（ディレクトリ / ZIP / OCI） | `[]` |
| `sbx.plugins`              | sbx      | プラグイン設定の配列 | `[]` |
| `sbx.plugins[].source`     | sbx      | ローカルパス（`~` 展開可）または GitHub URL（`user/repo`、`https://...`） | — |
| `sbx.plugins[].plugins`    | sbx      | インストールするプラグイン名。`["auto"]` でローカルの `plugins/` から自動検出 | — |
| `sbx.clone_base`           | sbx      | Git リポジトリ検索・`git clone` 先・sbx マウントのベースディレクトリ（深さ4まで探索） | `~/src` |
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

## ステータス一覧

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

## ブラウザ自動フォーカス

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

## プロジェクト設定

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

## 定型文カスタマイズ

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

## Worktree + sbx

Attach sbx の「New worktree」モードまたは CLI（`worktree create`）から、worktree 作成 + sbx アタッチ + Claude 自動起動が可能。

### sbx 側の必須セットアップ

1. Linux 用 hooks バイナリをビルド・配置（`make install-hook-linux`）
2. sbx 内の `~/.claude/settings.json` に Linux 用 hooks 設定を追加
   - hooks 設定は `sbx_post_create_cmds` に含めることで自動化できる（参考: [`../examples/sbx-setup.sh`](../examples/sbx-setup.sh)）。

`~/.claude-tabs` のマウントとシンボリックリンク作成はコードで自動実行されるため手動設定は不要。

## ターミナル設定

`iterm2` と `terminal`（Terminal.app）は内蔵プリセットがあり、設定不要で動作する。

```json
{ "terminal": "iterm2" }
```

`terminal_presets` に同名のエントリを書けば内蔵より優先される。
他のターミナル（Ghostty 等）を使う場合は `terminal_presets` に AppleScript を定義して `terminal` で名前を指定する。

テンプレート変数: `{{TTY}}`, `{{TEXT}}`, `{{CMDS}}`, `{{COMMAND}}`

詳細は [`../examples/config.json`](../examples/config.json) を参照。
