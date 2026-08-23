# Claude Code hooks 設定

`~/.claude/settings.json` の `hooks` セクションに以下を追加（`hooks-setup.json` を参照）。

## macOS（hook 実行環境が macOS の場合）

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

## Linux（sbx など hook 実行環境が Linux の場合）

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
