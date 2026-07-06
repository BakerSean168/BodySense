#!/bin/bash
# =============================================================================
# BodySense 远程开发服务器初始化脚本 (DigitalOcean 4H8G)
# =============================================================================
# 在 4H8G Droplet 上运行此脚本初始化远程开发环境：
#
#   scp scripts/setup-dev-server.sh DO-dev:/root/setup-dev-server.sh
#   ssh DO-dev "GITHUB_TOKEN=ghp_xxxxx /root/setup-dev-server.sh"
#
#   或交互式输入 Token:
#   ssh DO-dev "chmod +x /root/setup-dev-server.sh && /root/setup-dev-server.sh"
#
# GitHub Token 获取: https://github.com/settings/tokens → Generate new token → repo
#
# 此脚本将：
#   1. 安装 Docker Engine + Docker Compose
#   2. 安装 Node.js 24 + pnpm 11
#   3. 安装 Go 1.26
#   4. 安装 Python 3.13 + uv
#   5. 克隆仓库代码 (通过 GitHub Token 认证)
#   6. 启动开发基础设施 (Postgres + Redis via Docker)
#   7. 生成开发环境变量
# =============================================================================

set -euo pipefail

# ── 配置 ──────────────────────────────────────────────
DEV_DIR="/opt/bodysense-dev"
REPO_URL="https://github.com/T1moooo/BodySense.git"
REPO_BRANCH="dev"

# GitHub 认证: 优先使用 GITHUB_TOKEN 环境变量，其次尝试 SSH
# 用法: GITHUB_TOKEN=ghp_xxxxx ./setup-dev-server.sh
GITHUB_TOKEN="${GITHUB_TOKEN:-}"

echo "══════════════════════════════════════════════════"
echo "  BodySense 远程开发服务器初始化 (4H8G)"
echo "══════════════════════════════════════════════════"

# ── 检查 root 权限 ────────────────────────────────────
if [ "$EUID" -ne 0 ]; then
    echo "Error: 请以 root 用户运行此脚本"
    exit 1
fi

# ── 1. 系统更新 + 基础工具 ────────────────────────────
echo ">>> 安装基础工具..."
apt-get update -qq
apt-get install -y -qq curl wget git ca-certificates gnupg build-essential
echo "  Done."

# ── 2. 安装 Docker ────────────────────────────────────
if ! command -v docker &>/dev/null; then
    echo ">>> 安装 Docker Engine..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
    echo "  Docker $(docker --version | awk '{print $3}')"
else
    echo "  Docker 已安装: $(docker --version)"
fi

# ── 3. 安装 Node.js 24 + pnpm ────────────────────────
if ! command -v node &>/dev/null; then
    echo ">>> 安装 Node.js 24..."
    curl -fsSL https://deb.nodesource.com/setup_24.x | bash -
    apt-get install -y -qq nodejs
    echo "  Node.js $(node --version)"
else
    echo "  Node.js 已安装: $(node --version)"
fi

if ! command -v pnpm &>/dev/null; then
    echo ">>> 安装 pnpm..."
    corepack enable && corepack prepare pnpm@11 --activate
    echo "  pnpm $(pnpm --version)"
else
    echo "  pnpm 已安装: $(pnpm --version)"
fi

# ── 4. 安装 Go 1.26 ──────────────────────────────────
GO_VERSION="1.26.0"
if ! command -v go &>/dev/null; then
    echo ">>> 安装 Go ${GO_VERSION}..."
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile.d/go.sh
    echo 'export GOPATH=$HOME/go' >> /etc/profile.d/go.sh
    echo 'export PATH=$PATH:$GOPATH/bin' >> /etc/profile.d/go.sh
    chmod +x /etc/profile.d/go.sh
    export PATH=$PATH:/usr/local/go/bin
    echo "  Go $(go version | awk '{print $3}')"
else
    echo "  Go 已安装: $(go version)"
fi

# ── 5. 安装 Python 3.13 + uv ─────────────────────────
if ! command -v python3.13 &>/dev/null && ! python3 --version 2>/dev/null | grep -q "3.13"; then
    echo ">>> 安装 Python 3.13..."
    apt-get install -y -qq software-properties-common
    add-apt-repository -y ppa:deadsnakes/ppa
    apt-get update -qq
    apt-get install -y -qq python3.13 python3.13-venv python3.13-dev
    echo "  Python $(python3.13 --version)"
else
    echo "  Python 已安装: $(python3 --version)"
fi

if ! command -v uv &>/dev/null; then
    echo ">>> 安装 uv..."
    curl -LsSf https://astral.sh/uv/install.sh | sh
    source $HOME/.local/bin/env 2>/dev/null || true
    echo "  uv $(uv --version)"
else
    echo "  uv 已安装: $(uv --version)"
fi

# ── 6. 安装 tesseract-ocr (AI service 依赖) ──────────
echo ">>> 安装 tesseract-ocr..."
apt-get install -y -qq tesseract-ocr tesseract-ocr-chi-sim
echo "  Done."

# ── 7. 克隆仓库 ──────────────────────────────────────
if [ ! -d "${DEV_DIR}/.git" ]; then
    # 构造认证 URL
    if [ -n "${GITHUB_TOKEN}" ]; then
        AUTH_URL="https://x-access-token:${GITHUB_TOKEN}@github.com/T1moooo/BodySense.git"
        echo ">>> 使用 GitHub Token 认证克隆..."
    else
        echo ">>> 请输入 GitHub Personal Access Token (权限: repo)"
        echo "    获取地址: https://github.com/settings/tokens"
        read -sp "    Token: " GITHUB_TOKEN
        echo ""
        if [ -z "${GITHUB_TOKEN}" ]; then
            echo "Error: 未提供 Token，无法克隆私有仓库"
            exit 1
        fi
        AUTH_URL="https://x-access-token:${GITHUB_TOKEN}@github.com/T1moooo/BodySense.git"
    fi

    echo ">>> 克隆仓库到 ${DEV_DIR}..."
    git clone -b "${REPO_BRANCH}" "${AUTH_URL}" "${DEV_DIR}"

    # 配置 git credential 以便后续 pull 不需要重新输入
    cd "${DEV_DIR}"
    git config credential.helper "store --file=/root/.git-credentials"
    echo "https://x-access-token:${GITHUB_TOKEN}@github.com" > /root/.git-credentials
    chmod 600 /root/.git-credentials

    # 清除 URL 中的 token (安全)
    git remote set-url origin "${REPO_URL}"
else
    echo ">>> 更新仓库..."
    cd "${DEV_DIR}" && git pull origin "${REPO_BRANCH}" || true
fi

cd "${DEV_DIR}"

# ── 8. 生成开发环境变量 ──────────────────────────────
if [ ! -f "${DEV_DIR}/.env.development" ]; then
    echo ">>> 生成 .env.development..."

    JWT_SECRET_KEY=$(openssl rand -base64 48)

    cat > "${DEV_DIR}/.env.development" << EOF
# BodySense 开发环境变量
# 生成时间: $(date -u +"%Y-%m-%dT%H:%M:%SZ")

DB_USER=bodysense
DB_PASSWORD=bodysense123
DB_NAME=bodysense
DB_PORT=5432

REDIS_PASSWORD=bodysense123

JWT_SECRET_KEY=${JWT_SECRET_KEY}

# OpenRouter API Key — 请手动填写
OPENROUTER_API_KEY=
LLM_API_KEY=
EMBEDDING_API_KEY=

# 默认开发配置
LLM_PROVIDER=openrouter
LLM_MODEL=openai/gpt-oss-120b:free
EMBEDDING_PROVIDER=hashing
EMBEDDING_DIMENSIONS=384
ASK_USER_ENABLED=false
EOF

    echo "  .env.development 已生成"
else
    echo "  .env.development 已存在"
fi

# ── 9. 安装项目依赖 ──────────────────────────────────
echo ">>> 安装前端依赖..."
pnpm install --reporter=append-only || echo "  Warning: pnpm install 有错误，可稍后手动运行"

echo ">>> 安装 Go 依赖..."
cd apps/api && go mod download && cd ../..
echo "  Done."

echo ">>> 安装 Python 依赖..."
cd apps/ai-service && uv sync --no-dev --extra ocr && cd ../..
echo "  Done."

# ── 10. 启动开发基础设施 ─────────────────────────────
cd "${DEV_DIR}"
echo ">>> 启动 Postgres + Redis (Docker)..."
docker compose -f docker/docker-compose.yml --profile dev up -d postgres-dev redis-dev
echo "  等待数据库就绪..."
sleep 10

# ── 11. 输出总结 ─────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════════"
echo "  远程开发环境初始化完成!"
echo "══════════════════════════════════════════════════"
echo ""
echo "开发目录: ${DEV_DIR}"
echo ""
echo "基础设施状态:"
docker compose -f docker/docker-compose.yml --profile dev ps
echo ""
echo "常用命令:"
echo "  启动全部服务:    docker compose -f docker/docker-compose.yml --profile dev up -d"
echo "  停止全部服务:    docker compose -f docker/docker-compose.yml --profile dev down"
echo "  仅启动基础设施:  docker compose -f docker/docker-compose.yml --profile dev up -d postgres-dev redis-dev"
echo ""
echo "热启动开发 (不容器化应用):"
echo "  前端:   cd apps/web && pnpm dev"
echo "  后端:   cd apps/api && go run ./cmd/server"
echo "  AI:     cd apps/ai-service && uv run uvicorn src.main:app --host 0.0.0.0 --port 8100 --reload"
echo ""
echo "VS Code Remote SSH:"
echo "  1. 安装 Remote - SSH 扩展"
echo "  2. 添加 SSH Host: ssh root@<DROPLET_IP>"
echo "  3. Connect to Host → 打开 ${DEV_DIR}"
echo ""
echo "TODO:"
echo "  - 编辑 ${DEV_DIR}/.env.development 填入 OPENROUTER_API_KEY"
echo "  - 确认 SSH key 认证已配置"
echo ""
echo "下一步 — 安装开发工具 (Zsh + tmux + Neovim + AstroNvim):"
echo "  chmod +x scripts/setup-dev-tools.sh && ./scripts/setup-dev-tools.sh"
