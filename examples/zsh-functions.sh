#!/bin/bash
# claude-tabs CLI 用 zsh 関数の参考例
#
# claude-tabs バイナリとの依存はない。
# Web UI の「New Worktree」と同等の操作をターミナルから行うためのコマンド例。
# 必要に応じてコピー・編集して .zshrc 等に追加して使う。
#
# 前提:
#   - ghq (リポジトリ管理)
#   - fzf (インタラクティブ選択)
#   - sbx (Docker sandbox)
#   - iTerm2 (macOS)
#
# 環境変数 (事前に設定が必要):
#   - CLAUDE_TABS_WORKTREE_BASE    — worktree 保存先（デフォルト: $(ghq root)/worktrees）
#   - CLAUDE_TABS_SBX_TEMPLATE     — sbx テンプレート（デフォルト: my-sbx:latest）
#   - CLAUDE_TABS_SBX_DEFAULT_MOUNTS — sbx デフォルトマウント（スペース区切り）
#   - CLAUDE_TABS_SBX_SETUP_CMD    — sbx 作成後のセットアップコマンド
#   - CLAUDE_TABS_CLAUDE_PLUGINS_DIRS — Claude plugins ディレクトリ（スペース区切り）

# wtcd: worktree 一覧から fzf で選択して cd
function wtcd() {
  local wt_base="${CLAUDE_TABS_WORKTREE_BASE:-$(ghq root)/worktrees}"
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
  local wt_base="${CLAUDE_TABS_WORKTREE_BASE:-$(ghq root)/worktrees}"
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

# wt-sbx: worktree 作成 + sbx 作成 + iTerm 新タブで Claude 起動
function wt-sbx() {
  local repo="$1"
  local branch="$2"

  if [[ -z "$repo" || -z "$branch" ]]; then
    echo "Usage: wt-sbx <repo> <branch>"
    return 1
  fi

  # ghq からリポジトリ検索
  local repo_path=$(ghq list -p | grep "/${repo}$" | head -1)
  if [[ -z "$repo_path" ]]; then
    echo "Repository not found: $repo"
    return 1
  fi

  local wt_base="${CLAUDE_TABS_WORKTREE_BASE:-$(ghq root)/worktrees}"
  local wt_path="${wt_base}/${repo}/${branch}"
  local sbx_name="${repo}-${branch}"
  local template="${CLAUDE_TABS_SBX_TEMPLATE:-my-sbx:latest}"

  # worktree 作成
  if [[ -d "$wt_path" ]]; then
    echo "Worktree already exists: $wt_path"
  else
    git -C "$repo_path" fetch origin
    if git -C "$repo_path" rev-parse "origin/${branch}" >/dev/null 2>&1; then
      git -C "$repo_path" worktree add "$wt_path" "origin/${branch}" || return 1
    elif git -C "$repo_path" rev-parse --verify "$branch" >/dev/null 2>&1; then
      echo "Using existing local branch: $branch"
      git -C "$repo_path" worktree add "$wt_path" "$branch" || return 1
    else
      echo "Creating new branch: $branch"
      git -C "$repo_path" worktree add "$wt_path" -b "$branch" || return 1
    fi
  fi

  # sbx 作成
  local paths=("$wt_path")
  if [[ -n "$CLAUDE_TABS_SBX_DEFAULT_MOUNTS" ]]; then
    for mount in ${(s: :)CLAUDE_TABS_SBX_DEFAULT_MOUNTS}; do
      paths+=("$mount")
    done
  fi

  sbx create --name "$sbx_name" -t "$template" claude "${paths[@]}" || return 1

  # セットアップコマンド実行
  if [[ -n "$CLAUDE_TABS_SBX_SETUP_CMD" ]]; then
    sbx exec "$sbx_name" sh -c "$CLAUDE_TABS_SBX_SETUP_CMD" 2>/dev/null
  fi

  # plugins インストール
  if [[ -n "$CLAUDE_TABS_CLAUDE_PLUGINS_DIRS" ]]; then
    for plugins_dir in ${(s: :)CLAUDE_TABS_CLAUDE_PLUGINS_DIRS}; do
      local marketplace_name=$(python3 -c "import json; print(json.load(open('${plugins_dir}/.claude-plugin/marketplace.json'))['name'])" 2>/dev/null)
      if [[ -n "$marketplace_name" ]]; then
        sbx exec "$sbx_name" claude plugins marketplace add "$plugins_dir" 2>/dev/null
        for plugin in "${plugins_dir}"/plugins/*/; do
          sbx exec "$sbx_name" claude plugins install "$(basename "$plugin")@${marketplace_name}" 2>/dev/null
        done
      fi
    done
  fi

  # iTerm2 新タブで Claude 起動
  osascript -e "
    tell application \"iTerm2\"
      tell current window
        create tab with default profile
        tell current session of current tab
          write text \"sbx run --name ${sbx_name} claude\"
        end tell
      end tell
    end tell
  "
}
