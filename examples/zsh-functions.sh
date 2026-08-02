#!/bin/bash
# claude-tabs CLI 用 zsh 関数の参考例
#
# 必要に応じてコピー・編集して .zshrc 等に追加して使う。
#
# 前提:
#   - claude-tabs バイナリ (~/.claude-tabs/bin/claude-tabs)
#   - ghq (リポジトリ管理)
#   - fzf (インタラクティブ選択)
#   - sbx (Docker sandbox)
#
# 設定:
#   - ~/.claude-tabs/config.json（詳細は README 参照）

# wt-sbx: worktree + sbx + Claude 起動（claude-tabs バイナリに委譲）
function wt-sbx() {
  if [[ -z "$1" || -z "$2" ]]; then
    echo "Usage: wt-sbx <repo> <branch>"
    return 1
  fi
  ~/.claude-tabs/bin/claude-tabs worktree create "$1" "$2"
}

# wtcd: worktree 一覧から fzf で選択して cd
function wtcd() {
  local config_json="$HOME/.claude-tabs/config.json"
  local wt_base=""
  if [[ -f "$config_json" ]]; then
    wt_base=$(python3 -c "import json,os; c=json.load(open('$config_json')); b=c.get('worktree_base',''); print(os.path.expanduser(b) if b else '')" 2>/dev/null)
  fi
  if [[ -z "$wt_base" ]]; then
    wt_base="$(ghq root)/worktrees"
  fi
  if [[ ! -d "$wt_base" ]]; then
    echo "No worktrees directory: $wt_base"
    return 1
  fi

  local selected=$(find "$wt_base" -mindepth 2 -maxdepth 2 -type d | sed "s|${wt_base}/||" | fzf --height 50% --preview "cd ${wt_base}/{} && echo '=== Branch ===' && git branch --show-current 2>/dev/null && echo -e '\n=== Recent Commits ===' && git log --oneline -5 --color=always 2>/dev/null")

  if [[ -n "$selected" ]]; then
    cd "${wt_base}/${selected}"
    echo "Moved to: ${wt_base}/${selected}"
  fi
}

# wtrm: worktree + sbx を fzf で選択して削除
function wtrm() {
  local config_json="$HOME/.claude-tabs/config.json"
  local wt_base=""
  if [[ -f "$config_json" ]]; then
    wt_base=$(python3 -c "import json,os; c=json.load(open('$config_json')); b=c.get('worktree_base',''); print(os.path.expanduser(b) if b else '')" 2>/dev/null)
  fi
  if [[ -z "$wt_base" ]]; then
    wt_base="$(ghq root)/worktrees"
  fi
  if [[ ! -d "$wt_base" ]]; then
    echo "No worktrees directory: $wt_base"
    return 1
  fi

  local selected=$(find "$wt_base" -mindepth 2 -maxdepth 2 -type d | sed "s|${wt_base}/||" | fzf --height 50% --preview "cd ${wt_base}/{} && echo '=== Branch ===' && git branch --show-current 2>/dev/null && echo -e '\n=== Status ===' && git status -s 2>/dev/null")

  if [[ -z "$selected" ]]; then
    return 0
  fi

  local wt_path="${wt_base}/${selected}"
  local repo=$(dirname "$selected")
  local branch=$(basename "$selected")
  local sbx_name="${repo}-${branch}"

  # sbx 削除 (存在すれば)
  if sbx ls -q 2>/dev/null | grep -qx "$sbx_name"; then
    echo "Removing sandbox: $sbx_name"
    sbx rm "$sbx_name"
  fi

  # worktree 削除
  echo "Removing worktree: $wt_path"
  git -C "$(ghq list -p | grep "/${repo}$" | head -1)" worktree remove "$wt_path" || rm -rf "$wt_path"
}
