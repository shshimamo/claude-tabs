#!/bin/bash
# sbx セットアップスクリプトの参考例
#
# 使い方:
#   1. このファイルをコピーして自分の環境に合わせて編集
#   2. sbx のマウントパスに含める (sbx_default_mounts 等)
#   3. config.json で設定:
#      "sbx_post_create_cmds": [["/path/to/sbx-setup.sh"]]

# ------------------------
# --- dotfiles の設定例 ---
# ------------------------

# sbx_default_mounts で dotfiles をマウントしておく
DOTFILES="$HOME/dotfiles"

if [ -d "$DOTFILES" ]; then
  # --- dotfiles リンク ---
  ln -sf "$DOTFILES/.zshrc" ~/.zshrc
fi

# -------------------------------------------
# --- Claude Code hooks (Linux 用) の設定例 ---
# -------------------------------------------

# ホスト用 hooks を削除して Linux 用に差し替える（jq 必要）
if command -v jq >/dev/null; then
  # sbx_default_mounts で claude-tabs をマウントしておく
  HOOKS_JSON="$HOME/claude-tabs/hooks-setup-linux.json"
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
