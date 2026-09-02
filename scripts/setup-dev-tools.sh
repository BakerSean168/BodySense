#!/bin/bash
# =============================================================================
# BodySense 通用远程开发工具初始化脚本
# =============================================================================
# 安装 Zsh + Oh My Zsh + tmux + Neovim + AstroNvim，与 WSL 环境保持一致。
#
# 用法:
#   scp scripts/setup-dev-tools.sh <dev-host>:/tmp/setup-dev-tools.sh
#   ssh <dev-host> "chmod +x /tmp/setup-dev-tools.sh && /tmp/setup-dev-tools.sh"
#
# 可安全重复运行（幂等）。
# =============================================================================

set -euo pipefail

# ── 配置 ──────────────────────────────────────────────
NVIM_CONFIG_REPO="https://github.com/BakerSean168/nvim-config.git"
NVIM_VERSION="v0.11.3"  # 最新稳定版，按需更新
NVIM_TARBALL="https://github.com/neovim/neovim/releases/download/${NVIM_VERSION}/nvim-linux-x86_64.tar.gz"

# 目标用户 (root 时配置 root，非 root 时配置当前用户)
TARGET_USER="${SUDO_USER:-$(whoami)}"
TARGET_HOME="$(eval echo "~${TARGET_USER}")"

echo "══════════════════════════════════════════════════"
echo "  BodySense 开发工具初始化"
echo "  目标用户: ${TARGET_USER} (${TARGET_HOME})"
echo "══════════════════════════════════════════════════"

# ── 1. Zsh + Oh My Zsh ────────────────────────────────
if ! command -v zsh &>/dev/null; then
    echo ">>> 安装 Zsh..."
    apt-get update -qq
    apt-get install -y -qq zsh
    echo "  Zsh $(zsh --version)"
else
    echo "  Zsh 已安装: $(zsh --version)"
fi

# Oh My Zsh
if [ ! -d "${TARGET_HOME}/.oh-my-zsh" ]; then
    echo ">>> 安装 Oh My Zsh..."
    # 非交互式安装
    sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended 2>/dev/null || true
    # 如果安装到了 /root 但目标用户不是 root，移动过去
    if [ "${TARGET_USER}" != "root" ] && [ -d "/root/.oh-my-zsh" ] && [ ! -d "${TARGET_HOME}/.oh-my-zsh" ]; then
        cp -r /root/.oh-my-zsh "${TARGET_HOME}/.oh-my-zsh"
        chown -R "${TARGET_USER}:${TARGET_USER}" "${TARGET_HOME}/.oh-my-zsh"
    fi
    echo "  Oh My Zsh 已安装"
else
    echo "  Oh My Zsh 已存在"
fi

# Zsh 自定义插件
ZSH_CUSTOM="${TARGET_HOME}/.oh-my-zsh/custom"
if [ ! -d "${ZSH_CUSTOM}/plugins/zsh-autosuggestions" ]; then
    echo ">>> 安装 zsh-autosuggestions..."
    git clone --depth=1 https://github.com/zsh-users/zsh-autosuggestions "${ZSH_CUSTOM}/plugins/zsh-autosuggestions"
fi
if [ ! -d "${ZSH_CUSTOM}/plugins/zsh-syntax-highlighting" ]; then
    echo ">>> 安装 zsh-syntax-highlighting..."
    git clone --depth=1 https://github.com/zsh-users/zsh-syntax-highlighting "${ZSH_CUSTOM}/plugins/zsh-syntax-highlighting"
fi

# 写入 .zshrc (与 WSL 一致)
echo ">>> 配置 .zshrc..."
cat > "${TARGET_HOME}/.zshrc" << 'ZSHRC'
# ====== Oh My Zsh ======
export ZSH="$HOME/.oh-my-zsh"
ZSH_THEME="robbyrussell"
plugins=(git docker zsh-autosuggestions zsh-syntax-highlighting)
source $ZSH/oh-my-zsh.sh

# ====== Editor ======
export EDITOR='nvim'
export VISUAL='nvim'

# ====== PATH ======
# Neovim (if installed to /opt)
[ -d /opt/nvim-linux-x86_64/bin ] && export PATH="/opt/nvim-linux-x86_64/bin:$PATH"
# Go
[ -d /usr/local/go/bin ] && export PATH="$PATH:/usr/local/go/bin"
[ -d "$HOME/go/bin" ] && export PATH="$PATH:$HOME/go/bin"
# Python uv
[ -f "$HOME/.local/bin/env" ] && source "$HOME/.local/bin/env"
# Node
[ -d "$HOME/.nvm" ] && export NVM_DIR="$HOME/.nvm" && [ -s "$NVM_DIR/nvm.sh" ] && source "$NVM_DIR/nvm.sh"

# ====== Node.js ======
export NODE_OPTIONS="--max-old-space-size=8192"

# ====== Aliases ======
alias ll='ls -alFh'
alias la='ls -A'
alias l='ls -CF'
alias vim='nvim'
alias vi='nvim'

# ====== BodySense 开发快捷命令 ======
alias bsdev='docker compose -f docker/docker-compose.yml --profile dev up -d postgres-dev redis-dev'
alias bsstop='docker compose -f docker/docker-compose.yml --profile dev down'
alias bsw='cd apps/web && pnpm dev'
alias bsa='cd apps/api && go run ./cmd/server'
alias bsai='cd apps/ai-service && uv run --extra ocr --extra pose --extra document-ocr python scripts/ensure_pose_model.py && uv run --extra ocr --extra pose --extra document-ocr uvicorn src.main:app --host 0.0.0.0 --port 8100 --reload'
alias bslogs='docker compose -f docker/docker-compose.yml --profile dev logs -f'
ZSHRC

# ── 2. tmux + TPM ─────────────────────────────────────
if ! command -v tmux &>/dev/null; then
    echo ">>> 安装 tmux..."
    apt-get install -y -qq tmux
    echo "  tmux $(tmux -V)"
else
    echo "  tmux 已安装: $(tmux -V)"
fi

# TPM (Tmux Plugin Manager)
if [ ! -d "${TARGET_HOME}/.tmux/plugins/tpm" ]; then
    echo ">>> 安装 TPM..."
    git clone --depth=1 https://github.com/tmux-plugins/tpm "${TARGET_HOME}/.tmux/plugins/tpm"
else
    echo "  TPM 已存在"
fi

# .tmux.conf (与 WSL 一致)
echo ">>> 配置 .tmux.conf..."
cat > "${TARGET_HOME}/.tmux.conf" << 'TMUXCONF'
# 开启鼠标支持 (极其重要，允许你用鼠标拖拽调整窗口大小、点击切换窗格)
set -g mouse on

# 开启 256 色和 True Color 支持 (保证 Neovim 的主题颜色不会变灰)
set -g default-terminal "tmux-256color"
set -ga terminal-overrides ",*256col*:Tc"

# 将前缀键从默认的 Ctrl+b 改为 Ctrl+a (更顺手)
unbind C-b
set -g prefix C-a
bind C-a send-prefix

# 使用 | 和 - 来垂直和水平分割窗口 (更直观)
bind | split-window -h
bind - split-window -v
unbind '"'
unbind %

# 重新加载配置的快捷键 (前缀键 + r)
bind r source-file ~/.tmux.conf \; display "Tmux 配置已刷新!"

# ==========================
# 插件管理
# ==========================
set -g @plugin 'tmux-plugins/tpm'
set -g @plugin 'tmux-plugins/tmux-sensible'
set -g @plugin 'tmux-plugins/tmux-resurrect'

# 告诉 resurrect 恢复哪些额外程序
set -g @resurrect-processes 'nvim'

# 初始化 TPM (必须放在文件最末尾!)
run '~/.tmux/plugins/tpm/tpm'
TMUXCONF

# ── 3. Neovim ─────────────────────────────────────────
if ! command -v nvim &>/dev/null; then
    echo ">>> 安装 Neovim ${NVIM_VERSION}..."
    mkdir -p /opt
    curl -fsSL "${NVIM_TARBALL}" -o /tmp/nvim.tar.gz
    tar -C /opt -xzf /tmp/nvim.tar.gz
    rm /tmp/nvim.tar.gz
    ln -sf /opt/nvim-linux-x86_64/bin/nvim /usr/local/bin/nvim
    echo "  Neovim $(nvim --version | head -1)"
else
    echo "  Neovim 已安装: $(nvim --version | head -1)"
fi

# ── 4. AstroNvim 配置 ─────────────────────────────────
if [ -d "${TARGET_HOME}/.config/nvim/.git" ]; then
    echo ">>> 更新 AstroNvim 配置..."
    cd "${TARGET_HOME}/.config/nvim" && git pull || true
elif [ -d "${TARGET_HOME}/.config/nvim" ]; then
    echo ">>> 备份现有 nvim 配置..."
    mv "${TARGET_HOME}/.config/nvim" "${TARGET_HOME}/.config/nvim.bak.$(date +%s)"
    echo ">>> 克隆 AstroNvim 配置..."
    git clone "${NVIM_CONFIG_REPO}" "${TARGET_HOME}/.config/nvim"
else
    echo ">>> 克隆 AstroNvim 配置..."
    mkdir -p "${TARGET_HOME}/.config"
    git clone "${NVIM_CONFIG_REPO}" "${TARGET_HOME}/.config/nvim"
fi

# ── 5. Git 配置 ───────────────────────────────────────
echo ">>> 配置 Git..."
git config --global user.email "bakersean@foxmail.com"
git config --global user.name "baker"
git config --global http.postBuffer 524288000
git config --global core.editor "nvim"
echo "  Git user: $(git config --global user.name) <$(git config --global user.email)>"

# ── 6. 文件权限修正 ──────────────────────────────────
if [ "${TARGET_USER}" != "root" ]; then
    echo ">>> 修正文件权限..."
    chown -R "${TARGET_USER}:${TARGET_USER}" \
        "${TARGET_HOME}/.oh-my-zsh" \
        "${TARGET_HOME}/.zshrc" \
        "${TARGET_HOME}/.tmux" \
        "${TARGET_HOME}/.tmux.conf" \
        "${TARGET_HOME}/.config/nvim" \
        "${TARGET_HOME}/.gitconfig" \
        2>/dev/null || true
fi

# ── 7. 设置 Zsh 为默认 Shell ─────────────────────────
CURRENT_SHELL="$(getent passwd "${TARGET_USER}" | cut -d: -f7)"
if [ "${CURRENT_SHELL}" != "$(which zsh)" ]; then
    echo ">>> 设置 Zsh 为 ${TARGET_USER} 的默认 Shell..."
    chsh -s "$(which zsh)" "${TARGET_USER}"
    echo "  已切换 (下次登录生效)"
else
    echo "  Zsh 已是默认 Shell"
fi

# ── 8. 安装 tmux 插件 ────────────────────────────────
echo ">>> 安装 tmux 插件 (首次)..."
"${TARGET_HOME}/.tmux/plugins/tpm/bin/install_plugins" 2>/dev/null || true

# ── 总结 ──────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════════"
echo "  开发工具安装完成!"
echo "══════════════════════════════════════════════════"
echo ""
echo "已安装:"
echo "  - Zsh $(zsh --version 2>/dev/null | awk '{print $2}')"
echo "  - Oh My Zsh (robbyrussell + git/docker/autosuggestions/syntax-highlighting)"
echo "  - tmux $(tmux -V 2>/dev/null | awk '{print $2}') (prefix=Ctrl+a, mouse=on, resurrect)"
echo "  - Neovim $(nvim --version 2>/dev/null | head -1 | awk '{print $2}')"
echo "  - AstroNvim v5 (从 ${NVIM_CONFIG_REPO})"
echo ""
echo "首次启动 Neovim 会自动安装插件 (lazy.nvim)，耐心等待："
echo "  nvim  # 自动安装所有 LSP + 插件"
echo ""
echo "BodySense 开发快捷命令 (已写入 .zshrc):"
echo "  bsdev    - 启动 Postgres + Redis"
echo "  bsstop   - 停止所有基础设施"
echo "  bsw      - 前端 pnpm dev"
echo "  bsa      - 后端 go run"
echo "  bsai     - AI 服务 uvicorn --reload"
echo "  bslogs   - 查看基础设施日志"
echo ""
echo "tmux 常用操作:"
echo "  Ctrl+a |   - 左右分屏"
echo "  Ctrl+a -   - 上下分屏"
echo "  Ctrl+a r   - 重载 tmux 配置"
echo "  Ctrl+a I   - 安装 tmux 插件"
