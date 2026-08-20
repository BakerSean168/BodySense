#!/bin/bash
# =============================================================================
# BodySense 服务器初始化脚本
# =============================================================================
# 在阿里云服务器上运行此脚本完成首次部署：
#
#   scp scripts/setup-server.sh ali:/root/setup-server.sh
#   ssh ali "chmod +x /root/setup-server.sh && /root/setup-server.sh"
#
# 此脚本将：
#   1. 安装 Docker Engine + Docker Compose
#   2. 登录阿里云 ACR (需要提供凭据)
#   3. 创建部署目录 /opt/bodysense
#   4. 克隆仓库代码
#   5. 生成 .env.production.local (自动生成密码)
#   6. 拉取镜像并启动全部服务
# =============================================================================

set -euo pipefail

# ── 配置 ──────────────────────────────────────────────
DEPLOY_DIR="/opt/bodysense"
REPO_URL="https://github.com/T1moooo/BodySense.git"

# ACR 凭据 — 通过环境变量传入或交互式输入
ACR_REGISTRY="${ACR_REGISTRY:-crpi-cv97phwhms6wy4as.cn-hangzhou.personal.cr.aliyuncs.com}"
ACR_USERNAME="${ACR_USERNAME:-}"
ACR_PASSWORD="${ACR_PASSWORD:-}"

echo "══════════════════════════════════════════════════"
echo "  BodySense 服务器初始化"
echo "══════════════════════════════════════════════════"

# ── 检查 root 权限 ────────────────────────────────────
if [ "$EUID" -ne 0 ]; then
    echo "❌ 请以 root 用户运行此脚本"
    exit 1
fi

# ── 1. 安装 Docker ────────────────────────────────────
if ! command -v docker &>/dev/null; then
    echo "📦 安装 Docker Engine..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
    echo "✅ Docker 安装完成: $(docker --version)"
else
    echo "✅ Docker 已安装: $(docker --version)"
fi

# ── 2. 获取 ACR 凭据 (如果未通过环境变量提供) ────────
if [ -z "$ACR_USERNAME" ] || [ -z "$ACR_PASSWORD" ]; then
    echo ""
    echo "🔐 需要阿里云 ACR 登录凭据"
    echo "   Registry: $ACR_REGISTRY"
    read -p "   用户名: " ACR_USERNAME
    read -sp "   密码: " ACR_PASSWORD
    echo ""
fi

# ── 3. 登录阿里云 ACR ────────────────────────────────
echo "🔐 登录阿里云 ACR..."
echo "$ACR_PASSWORD" | docker login --username "$ACR_USERNAME" --password-stdin "$ACR_REGISTRY"
echo "✅ ACR 登录成功"

# ── 4. 创建部署目录并克隆仓库 ────────────────────────
if [ ! -d "${DEPLOY_DIR}/.git" ]; then
    echo "📥 克隆仓库到 ${DEPLOY_DIR}..."
    git clone "${REPO_URL}" "${DEPLOY_DIR}"
else
    echo "📥 更新仓库..."
    cd "${DEPLOY_DIR}" && git pull origin main || true
fi

cd "${DEPLOY_DIR}"

# ── 5. 生成 .env.production.local ───────────────────
if [ ! -f "${DEPLOY_DIR}/.env.production.local" ]; then
    echo "🔑 生成生产环境密钥..."

    DB_PASSWORD=$(openssl rand -base64 24 | tr -dc 'a-zA-Z0-9' | head -c 32)
    REDIS_PASSWORD=$(openssl rand -base64 24 | tr -dc 'a-zA-Z0-9' | head -c 32)
    JWT_SECRET_KEY=$(openssl rand -base64 48)
    LITELLM_MASTER_KEY=$(openssl rand -hex 32)

    cat > "${DEPLOY_DIR}/.env.production.local" << EOF
# BodySense 生产环境敏感配置 (不提交 Git)
# 生成时间: $(date -u +"%Y-%m-%dT%H:%M:%SZ")

# 数据库密码
DB_PASSWORD=${DB_PASSWORD}

# Redis 密码
REDIS_PASSWORD=${REDIS_PASSWORD}

# JWT 密钥
JWT_SECRET_KEY=${JWT_SECRET_KEY}

# AI Service -> LiteLLM 内部网关认证密钥
LITELLM_MASTER_KEY=${LITELLM_MASTER_KEY}

# Model provider keys — 至少配置当前 gateway route 所需的一个可用 provider
MIMO_API_KEY=
OPENROUTER_API_KEY=
LLM_API_KEY=
EMBEDDING_API_KEY=
EOF

    chmod 600 "${DEPLOY_DIR}/.env.production.local"
    echo "✅ .env.production.local 已生成 (权限 600)"
    echo "⚠️  请手动编辑此文件填入 MIMO_API_KEY / OPENROUTER_API_KEY"
else
    echo "✅ .env.production.local 已存在"
fi

# Gateway migration is idempotent for servers created before LiteLLM existed.
if ! grep -q '^LITELLM_MASTER_KEY=' "${DEPLOY_DIR}/.env.production.local"; then
    echo "🔑 为现有部署生成 LiteLLM 内部网关密钥..."
    LITELLM_MASTER_KEY=$(openssl rand -hex 32)
    printf '\n# AI Service -> LiteLLM internal gateway auth\nLITELLM_MASTER_KEY=%s\n' \
        "$LITELLM_MASTER_KEY" >> "${DEPLOY_DIR}/.env.production.local"
    chmod 600 "${DEPLOY_DIR}/.env.production.local"
fi

# ── 6. 拉取镜像并启动服务 ───────────────────────────
echo "🚀 拉取镜像..."
docker compose -f docker/docker-compose.prod.yml \
    --env-file .env.production \
    --env-file .env.production.local \
    pull

echo "🚀 启动服务..."
docker compose -f docker/docker-compose.prod.yml \
    --env-file .env.production \
    --env-file .env.production.local \
    up -d

# ── 7. 等待服务就绪 ──────────────────────────────────
echo "⏳ 等待服务启动..."
sleep 15

echo "══════════════════════════════════════════════════"
echo "  ✅ 部署完成！"
echo "══════════════════════════════════════════════════"
echo ""
echo "服务状态："
docker compose -f docker/docker-compose.prod.yml ps
echo ""
echo "访问地址: http://$(curl -s ifconfig.me)"
echo "API 健康检查: http://$(curl -s ifconfig.me)/api/health"
echo ""
echo "常用命令："
echo "  查看日志: docker compose -f docker/docker-compose.prod.yml logs -f"
echo "  重启服务: docker compose -f docker/docker-compose.prod.yml restart"
echo "  停止服务: docker compose -f docker/docker-compose.prod.yml down"
echo ""
echo "⚠️  请编辑 ${DEPLOY_DIR}/.env.production.local 填入 MIMO_API_KEY / OPENROUTER_API_KEY"
echo "⚠️  然后重启: docker compose -f docker/docker-compose.prod.yml --env-file .env.production --env-file .env.production.local restart"
