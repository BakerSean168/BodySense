# BodySense

BodySense 是一款体态健康 AI 助手应用，通过 AI 视觉分析帮助用户评估和改善体态问题。

## Tech Stack

- **Frontend**: React 19 + TypeScript 6 + Vite 8 + shadcn/ui + Tailwind CSS 4
- **Backend**: Go 1.26 + Gin 1.12 + GORM
- **AI Service**: Python 3.13 + FastAPI + LangChain v1
- **Database**: PostgreSQL 16 (pgvector) + Redis 7
- **Monorepo**: Nx + pnpm 11
- **Deployment**: Docker Compose + Caddy + Watchtower

## Quick Start

```bash
# 1. Clone and install
git clone <repo-url> && cd BodySense
pnpm install

# 2. Copy environment file
cp .env.example .env

# 3. Start dev infrastructure
docker compose -f docker/docker-compose.yml --profile dev up -d

# 4. Start development servers
pnpm dev
```

## Project Structure

```
apps/
  web/          ← React frontend (pnpm)
  api/          ← Go backend (go mod)
  ai-service/   ← Python AI service (uv)
docker/         ← Docker Compose orchestration
tools/
  agent-skills/ ← Project-specific agent skills
scripts/        ← Dev utility scripts
docs/           ← Project documentation (PRD, tech specs, prototypes)
```

## AI Collaboration

This project uses an AI-first development workflow. See [`AGENT.md`](./AGENT.md) for:

- Truth priority and project conventions
- Git strategy and commit format
- MCP server configuration
- Agent skills and validation commands

## Scripts

| Command | Description |
|---------|-------------|
| `pnpm dev` | Start web + api dev servers |
| `pnpm build` | Build all apps |
| `pnpm lint` | Lint all projects |
| `pnpm typecheck` | Type-check all projects |
| `pnpm test` | Run all tests |

## License

Private - All rights reserved.
