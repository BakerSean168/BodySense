#!/bin/bash
#
# 安装 Git hooks 到本地 .git/hooks/ 目录
#

HOOKS_DIR="$(cd "$(dirname "$0")" && pwd)/hooks"
GIT_HOOKS_DIR="$(git rev-parse --show-toplevel)/.git/hooks"

echo "安装 Git hooks..."

for hook in "$HOOKS_DIR"/*; do
  hook_name=$(basename "$hook")
  target="$GIT_HOOKS_DIR/$hook_name"
  cp "$hook" "$target"
  chmod +x "$target"
  echo "  ✓ $hook_name"
done

echo ""
echo "Git hooks 安装完成！"
