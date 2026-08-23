#!/bin/bash
# ------------------------------------------
# --- Claude Code hooks (Linux 用) の設定 ---
# ------------------------------------------

# ホスト用 hooks を削除して Linux 用に差し替える（jq 必要）
if command -v jq >/dev/null; then
  HOOKS_JSON="{CLAUDE_TABS_PATH}/hooks-setup-linux.json"
  if [ -f "$HOOKS_JSON" ]; then
    # ホスト用 claude-tabs hooks があれば削除
    jq '
      .hooks //= {} |
      .hooks |= with_entries(
        .value |= map(select(.hooks | all(.command | contains("claude-tabs") | not)))
      )
    ' ~/.claude/settings.json > ~/.claude/settings.json.tmp && mv ~/.claude/settings.json.tmp ~/.claude/settings.json

    # Linux 用 hooks を追加
    jq --slurpfile h "$HOOKS_JSON" '
      reduce ($h[0].hooks | to_entries[]) as $e (.;
        .hooks[$e.key] = ((.hooks[$e.key] // []) + $e.value)
      )
    ' ~/.claude/settings.json > ~/.claude/settings.json.tmp && mv ~/.claude/settings.json.tmp ~/.claude/settings.json
  fi
fi

echo "setup complete"
