# BodySense

> **An Agent-powered personal body-state workspace** for capturing body changes, maintaining a single current comprehensive diagnosis, and managing independently authorized training plans.

<p align="left">
  <strong>Language:</strong> English · <a href="./docs/zh-CN/README.md">Simplified Chinese</a>
</p>

<p align="left">
  <a href="https://body.bakersean.top"><strong>Live App</strong></a> ·
  <a href="https://bakersean168.github.io/BodySense/"><strong>Project Page</strong></a> ·
  <a href="./docs/product/README.md"><strong>Product Spec</strong></a> ·
  <a href="./docs/architecture/README.md"><strong>Architecture</strong></a>
</p>

BodySense keeps long-term body information as structured, time-aware data. A clear user statement about their own body is recorded automatically, while ambiguities are clarified through interactive tools. The Agent can help analyze specific conditions within one comprehensive Diagnosis and suggest adjustments to individual training plans. Formal Diagnosis changes require the user's request or authorization; training content changes require a direct request or acceptance of an Agent proposal.

The interface uses a persistent interactive 3D body canvas with movable Chat and business windows for Person, Diagnosis, TrainingPlan and Trainer. The normal experience shows current results rather than internal revision history.

> [!IMPORTANT]
> BodySense supports personal health-information organization and evidence-aware explanation. It is not a medical device and does not replace qualified medical diagnosis or treatment.

## Business model

- **BodyState** — one durable state per user, with clear self-reports saved automatically and correction distinguished from actual progression.
- **Diagnosis** — one comprehensive current result, made up of condition entries that can be updated independently within an atomic overall revision.
- **TrainingPlan** — multiple independent current plans, changed only when the user requests or explicitly accepts a proposed modification.
- **Agent** — natural-language interaction, targeted evidence gathering, clarification, scoped diagnosis analysis and plan-impact review.
- **3D workspace** — persistent body canvas, context tools and movable windows, consistent on desktop and mobile.

For detailed semantics, see the [product specification](./docs/product/README.md).

## Why it is interesting

BodySense focuses less on “how to call a model” and more on **how AI output becomes durable, traceable, reviewable product state**.

| Engineering concern | BodySense approach |
| --- | --- |
| Long-running AI interaction | Run / Turn / checkpoint + durable event log |
| Human-in-the-loop | structured `ask_user` interrupt / resume |
| AI ↔ business state | Go owns durable business state; AI runtime produces validated proposals/events |
| Retrieval | pgvector-backed evidence retrieval with provenance |
| Model boundary | standalone LiteLLM gateway; application code does not own physical provider routing |
| Diagnosis governance | immutable configuration identity, replay/eval, hard safety gates |
| Production delivery | coherent immutable release artifacts + revision-checked rollout |

## Architecture

```text
React Web
   │  HTTP + SSE
   ▼
Go API / domain authority
   │
   ├── PostgreSQL + pgvector
   ├── Redis
   └── Python AI Service
          │
          ├── FastAPI
          ├── LangGraph durable runtime
          ├── PydanticAI typed agents
          └── LiteLLM model gateway
```

The repository is an Nx + pnpm monorepo:

```text
apps/
├── web/          React 19 + TypeScript + Vite
├── api/          Go + Gin + GORM
└── ai-service/   Python + FastAPI + LangGraph + PydanticAI

docker/           Local / production orchestration
scripts/          Validation and release utilities
docs/             Product specs, ADRs, architecture and plans
tools/            Project-specific engineering tools
```

For the current runtime and deployment boundaries, see [`docs/architecture/deployment-architecture.md`](./docs/architecture/deployment-architecture.md).

## Tech stack

- **Frontend:** React 19, TypeScript 6, Vite 8, Tailwind CSS 4, shadcn/ui
- **Backend:** Go 1.26, Gin, GORM
- **AI:** Python 3.13, FastAPI, LangGraph, PydanticAI
- **Data:** PostgreSQL + pgvector, Redis
- **Model gateway:** LiteLLM
- **Workspace:** Nx 23, pnpm 11
- **Quality:** Vitest, Go test, Pytest, Playwright, Ruff/Pyright, ESLint
- **Delivery:** GitHub Actions, Docker/Compose, Alibaba Cloud ACR + ECS

## Quick start

### Prerequisites

- Node.js 24+
- pnpm 11+
- Go 1.26+
- Python 3.13+
- Docker with Compose

### Run locally

```bash
git clone https://github.com/BakerSean168/BodySense.git
cd BodySense
pnpm install
pnpm dev
```

`pnpm dev` is the canonical interactive direct-dev entrypoint. It idempotently ensures the Docker-only development infrastructure is healthy, loads `.env.dev.local` when present, and then runs Web/API/AI on the host with hot reload.

The infrastructure is intentionally longer-lived than the application processes:

```bash
pnpm dev:infra:status  # PostgreSQL 18 + pgvector, Redis, LiteLLM
pnpm dev:infra:up      # create/start and wait for health
pnpm dev:infra:logs    # infrastructure logs
pnpm dev:infra:down    # explicit teardown; persistent volumes are preserved
```

The default direct-dev ports are Web `20100`, API `20101`, AI `20102`, PostgreSQL `20110`, Redis `20111`, and LiteLLM `20112`. Infrastructure containers use `restart: unless-stopped`, so a normal Docker/host restart brings them back without requiring the application processes to stay running.

Put host-specific overrides and provider credentials in the gitignored `.env.dev.local`; `scripts/dev-env.sh` supplies non-secret development defaults, so copying `.env.example` to `.env` is not required. The direct-dev Web lane enables Vite Bundled Dev by default so a workstation reaching GCP over Tailscale/SSH receives a small set of bundled development assets instead of a large native-ESM request waterfall while keeping HMR. Set `BODYSENSE_VITE_BUNDLED_DEV=false` in `.env.dev.local` to temporarily return to classic Vite for diagnostics; tests remain on classic Vite.

Run a single surface when you do not need the full stack:

```bash
pnpm dev:web
pnpm dev:api
pnpm dev:ai
```

## Validation

The repository keeps local and CI validation close to the same contract.

```bash
pnpm lint
pnpm typecheck
pnpm test
pnpm e2e
pnpm verify:release
```

`pnpm verify:release` is the broad repository release gate. CI also validates published database migration history and longitudinal browser journeys before a release becomes production-eligible.

### API contracts and Postman

The browser-facing REST contract has one source of truth: [`packages/contracts/openapi/bodysense.v1.openapi.yaml`](./packages/contracts/openapi/bodysense.v1.openapi.yaml). The Postman collection is generated from that OpenAPI rather than maintained as a second specification:

```bash
pnpm postman:generate          # regenerate collection + safe environment templates
pnpm postman:verify            # freshness + 95/95 OpenAPI route parity
pnpm postman:validate-native   # optional: validate with the installed Postman CLI
```

See [`postman/README.md`](./postman/README.md) for environment handling, internal-service boundaries, and Native Git / Postman Cloud binding.

## Production

The current production application is available at **[body.bakersean.top](https://body.bakersean.top)**.

The production path is intentionally separated from the GitHub Pages project page:

- **GitHub Pages** explains the project and its engineering story.
- **Alibaba Cloud ECS** runs the actual product.
- **Alibaba Cloud ACR** stores production OCI artifacts.
- **GitHub Actions** owns CI, release creation, and immutable image publication.

See [`docs/architecture/deployment-architecture.md`](./docs/architecture/deployment-architecture.md) for the current deployment contract.

## Documentation

- [Documentation index (Chinese)](./docs/README.md) — what is current, where each topic lives, what is archived
- [Product overview](./docs/product/README.md)
- [Product requirements](./docs/product/requirements.md)
- [Domain contracts](./docs/product/domain.md)
- [Agent interactions](./docs/product/agent-workflows.md)
- [UI behavior](./docs/product/interface.md)
- [Acceptance](./docs/product/acceptance.md)
- [Glossary & Penpot name mapping](./docs/product/glossary.md)
- [Architecture](./docs/architecture/README.md)
- [Repository conventions](./AGENT.md)

## Repository status & license

BodySense is a **public source repository**, but it currently does **not** include an open-source license. Public visibility alone does not grant permission to copy, modify, or redistribute the code; copyright remains with the repository owner unless a license is added later.
