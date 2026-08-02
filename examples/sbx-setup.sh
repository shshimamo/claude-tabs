#!/bin/bash
# sbx セットアップスクリプトの参考例
# config.json の sbx_post_create_cmd にこのスクリプトのコマンドを設定して使う
#
# 使い方:
#   1. このファイルをコピーして自分の環境に合わせて編集
#   2. sbx のマウントパスに含める (sbx_default_mounts 等)
#   3. config.json で設定:
#      "sbx_post_create_cmd": ["/path/to/sbx-setup.sh"]

# --- 設定 ---
# dotfiles のマウント先パス（SBX_DEFAULT_MOUNTS でマウントしておく）
DOTFILES="$HOME/dotfiles"

# --- dotfiles リンク ---
if [ -d "$DOTFILES" ]; then
  ln -sf "$DOTFILES/.zshrc" ~/.zshrc
  ln -sf "$DOTFILES/.gitconfig" ~/.gitconfig
  ln -sf "$DOTFILES/.gitignore_global" ~/.gitignore_global

  mkdir -p ~/.config
  # 例: starship prompt
  # ln -sf "$DOTFILES/starship.toml" ~/.config/starship.toml
fi

# --- Claude 設定 ---
# claude/settings.json をコピー（sandbox 固有の hooks を追記するため）
# mkdir -p ~/.claude
# cp "$DOTFILES/claude/settings.json" ~/.claude/settings.json

# --- Claude Code hooks (Linux 用) ---
# ホスト用 hooks を削除して Linux 用に差し替える例:
# if command -v jq >/dev/null; then
#   HOOKS_JSON="/path/to/claude-tabs/hooks-setup-linux.json"
#   if [ -f "$HOOKS_JSON" ]; then
#     # ホスト用 claude-tabs hooks を削除
#     jq '
#       .hooks //= {} |
#       .hooks |= with_entries(
#         .value |= map(select(.hooks | all(.command | contains("claude-tabs") | not)))
#       )
#     ' ~/.claude/settings.json > ~/.claude/settings.json.tmp && mv ~/.claude/settings.json.tmp ~/.claude/settings.json
#
#     # Linux 用 hooks を追加
#     jq --slurpfile h "$HOOKS_JSON" '
#       reduce ($h[0].hooks | to_entries[]) as $e (.;
#         .hooks[$e.key] = ((.hooks[$e.key] // []) + $e.value)
#       )
#     ' ~/.claude/settings.json > ~/.claude/settings.json.tmp && mv ~/.claude/settings.json.tmp ~/.claude/settings.json
#   fi
# fi

echo "setup complete"
