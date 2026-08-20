# BodySense DigitalOcean practice environment (non-sensitive)

NODE_ENV=production
TZ=Asia/Shanghai

REGISTRY=registry.digitalocean.com/body-sense-docker-repo
WEB_TAG=prod-latest
API_TAG=prod-latest
AI_TAG=prod-latest

APP_DOMAIN=bodydoo.bakersean.top
ACME_EMAIL=admin@bakersean.top

DB_PORT=25061
DB_NAME=bodysense
DB_SSLMODE=require

REDIS_PORT=25061
REDIS_TLS=true

API_PORT=8080
API_HOST=0.0.0.0
JWT_ACCESS_TTL_HOURS=168
JWT_REFRESH_TTL_HOURS=720
CORS_ORIGINS=https://bodydoo.bakersean.top

AI_SERVICE_PORT=8100
EMBEDDING_PROVIDER=hashing
EMBEDDING_DIMENSIONS=384
EMBEDDING_BASE_URL=https://openrouter.ai/api/v1
ASR_PROVIDER=asr_api
ASK_USER_ENABLED=true
AI_SERVICE_URL=http://ai-service:8100

VITE_WS_URL=wss://bodydoo.bakersean.top/ws
CONNECT_TO_PRODUCTION_VPS=ssh DO-bodysense-deploy
