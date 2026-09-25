# BodySense Full-Stack Upgrade & Remote Development Modernization Plan — 2026-09-24

> Status: **ACTIVE / CHECKPOINTS 1-4 LOCAL ACCEPTED / CHECKPOINT 5 COMPATIBLE SUBSET ACCEPTED / CHECKPOINTS 6-10 LOCAL ACCEPTED / LATER PHASES PENDING**
>
> Owner: BodySense repository
>
> Baseline audited on GCP Dev: `main@eec736b9c`, clean worktree, local branch ahead of `origin/main` by 7 commits.
>
> Goal: complete one coordinated technology-stack upgrade program while preserving BodySense product/runtime contracts, and adopt the proven MemoFlow Vite Bundled Dev pattern for remote GCP development.
>
> Important: this plan began as documentation-only. **Checkpoints 1-4 (BS-UPG-000/010/020/021/030/040/041), Checkpoint 6 (BS-UPG-060), Checkpoint 7 (BS-UPG-070), atomic Checkpoint 8 (BS-UPG-071/072), Checkpoint 9 (BS-UPG-080), and Checkpoint 10 (BS-UPG-081) are locally implemented and accepted on GCP Dev; Checkpoint 5 (BS-UPG-050) has its compatible subset accepted with Vitest 5 / GraphQL 17 explicitly held by upstream peer ranges. The Body Explorer staging visual pointer-hit gate remains intentionally deferred to promotion. No live staging or production deployment has been performed, and later upgrade phases remain pending.**

---

## 0. Executive decision summary

BodySense is already on a relatively modern stack, but the repository currently has four distinct kinds of drift:

1. **Remote Web development still uses classic Vite native ESM**, causing ~304 browser-facing `/src/` requests on the measured initial page load and making GCP → workstation RTT materially visible.
2. **Repository/runtime toolchain declarations are ahead of the GCP host**, especially Go: `go.mod` and CI/Docker use Go 1.26 while GCP Dev still runs Go 1.22.2.
3. **Several application/framework dependencies have safe minor/patch updates available**, while a smaller set are major or behavior-sensitive upgrades that require isolated validation.
4. **Infrastructure images are unevenly modernized**: PostgreSQL is already on 18, while Redis remains 7.x, nginx remains 1.27, and LiteLLM is behind the current release line.

The upgrade will therefore be executed as **one program with multiple independently verifiable batches**, not as one undifferentiated `pnpm update` / `uv lock --upgrade` / `go get -u ./...`.

### Target principles

- Preserve the existing BodySense architecture and product contracts.
- Do not rewrite imports/barrels merely to optimize remote development.
- Use **Vite Bundled Dev only for the persistent remote host-dev lane**; keep classic Vite available for tests and diagnostics.
- Keep **Node 24 LTS**, **Python 3.13**, **PostgreSQL 18**, and **pnpm 11** during this program.
- Upgrade **Go host runtime to 1.26.8** to match repository/CI/runtime expectations.
- Keep **TypeScript 6 as ecosystem-compatible baseline** while retaining the existing TypeScript 7 lane; do not make TS7 the sole toolchain while `typescript-eslint` still declares `<6.1`.
- Upgrade Redis from 7.x to the current Redis 8 line only behind persistence/rollback validation.
- Treat PydanticAI/OpenAI, Three/R3F, Vitest 5, GraphQL 17, Redis 8, and production base images as **high-signal upgrade boundaries**, each with dedicated focused checks before the repository-wide gate.
- No production deployment until staging and the full local release gate are green.

---

# 1. Audit baseline

## 1.1 Repository and host

At plan creation:

```text
repository: /home/dev/projects/bodysense
branch:     main
HEAD:       eec736b9c
status:     clean
tracking:   origin/main [ahead 7]
```

GCP Dev host:

```text
Git:        2.43.0
Node:       24.19.0
pnpm:       11.17.0
Python CLI: 3.12.3
AI venv:    Python 3.13.14
uv:         0.12.1
Go host:    1.22.2
Docker:     29.7.2
```

Repository declarations/runtime images:

```text
Node:        >=24
pnpm:        >=11
Go module:   go 1.26.0
CI Go:       1.26
API image:   golang:1.26-alpine
AI image:    python:3.13-slim (digest pinned)
PostgreSQL:  pgvector/pgvector:pg18
Redis:       redis:7-alpine
LiteLLM:     v1.97.0
Web runtime: nginx:1.27-alpine
```

## 1.2 Measured remote-development problem

Classic BodySense Vite dev server:

```text
total browser requests:       ~345
/src/ source requests:        ~304
Vite optimized-dep requests:  ~33
contracts source requests:    ~5
```

Measured page navigation:

```text
near-local RTT:
DOMContentLoaded ~1.47 s

simulated 80 ms network latency:
DOMContentLoaded ~4.16 s
```

This is the same underlying failure mode already addressed in MemoFlow:

```text
large browser-facing native-ESM graph
        ×
remote RTT
        =
module request waterfall / slow remote dev
```

The correct fix is a host-dev serving strategy, not a broad import-style rewrite.

## 1.3 Current strengths that must remain

BodySense already has:

- route/page lazy loading;
- a dedicated lazy boundary for `BodyExplorer3D`;
- explicit production chunk budgets;
- dependency pre-bundling for Base UI/Floating UI;
- a small Web → workspace source boundary, mainly `@bodysense/contracts`;
- PostgreSQL 18 deployment topology;
- immutable/candidate delivery machinery;
- local deployment validation;
- typed Go/Python/React contract boundaries;
- extensive Web/Go/Python tests.

These are protected, not redesign targets.

---

# 2. Upgrade decision matrix

Versions below are the candidate targets verified during the 2026-09-24 audit. Exact image digests must be resolved again at implementation time.

## 2.1 Foundation/runtime toolchain

| Component | Current | Target | Decision |
| --- | ---: | ---: | --- |
| Git on GCP Dev | 2.43.0 | 2.55.0 | Upgrade host tooling if the user meant Git literally; independent from Vite |
| Node | 24.19.0 | 24.21.x LTS | Upgrade within Node 24 LTS; **do not move to Node 26 Current** |
| pnpm | 11.17.0 | 11.27.1 | Upgrade within v11; **do not move to pnpm 12 in this program** |
| Go host | 1.22.2 | 1.26.8 | Required convergence with `go.mod`, CI, and Docker |
| Go module line | 1.26.0 | 1.26.x | Keep language/toolchain line on Go 1.26 |
| Python AI runtime | 3.13.14 | 3.13.15 | Patch upgrade only |
| Python feature line | 3.13 | 3.13 | **Do not move to Python 3.14 yet** |
| uv | 0.12.1 | current compatible patch/minor | Upgrade with lock regeneration only if no resolver regression |
| PostgreSQL | 18 / pg18 | PostgreSQL 18.6-compatible pgvector image | Stay on PG18; no PG19 beta |
| Docker Engine host | 29.7.2 | keep unless host audit finds a required patch | No unrelated daemon major change |

### Why Node 26 / Python 3.14 / pnpm 12 are excluded

They are newer feature/major lines, but moving them simultaneously with Vite, AI, 3D, Redis, and test-framework changes would increase blast radius without solving a current BodySense problem.

The objective is **modern stable convergence**, not maximum version-number churn.

## 2.2 Web build/tooling

| Component | Current | Target |
| --- | ---: | ---: |
| Vite | 8.1.5 | 8.3.0 |
| `@vitejs/plugin-react` | 6.0.4 | 6.1.1 |
| Nx / `@nx/*` | 23.1.0 | 23.2.1 |
| TypeScript baseline alias | 6.0.2 | latest 6.0.x compatible patch |
| TypeScript 7 lane | 7.0.2 | 7.0.2 unless a newer compatible 7.0 patch exists at implementation |
| ESLint | 10.8.0 | 10.11.0 |
| `typescript-eslint` | 8.65.0 | 8.70.1 |
| Prettier | 3.9.6 | 3.9.9 |
| esbuild | 0.28.2 | current Vite-8-compatible patch |
| Vitest | 4.1.10 | 5.0.1 |
| Playwright | 1.62.1 | 1.63.0 |
| happy-dom | 20.11.1 | 20.14.5 |

### TypeScript policy

Current BodySense already has a dual setup:

- root `typescript` alias → TypeScript 6;
- `typescript7` → TypeScript 7;
- selected project typecheck targets already invoke the TS7 binary.

This plan preserves that split.

`typescript-eslint@8.70.1` still declares TypeScript `>=4.8.4 <6.1.0`, so TS7 must **not** become the sole lint/parser toolchain yet.

## 2.3 React/application runtime

| Component | Current | Target |
| --- | ---: | ---: |
| React | 19.2.8 | 19.3.0 |
| React DOM | 19.2.8 | 19.3.0 |
| React Router | 8.3.0 | 8.4.0 |
| TanStack React Query | 5.101.4 | 5.103.2 |
| Base UI | 1.6.0 | 1.8.0 |
| Zustand | 5.0.14 | 5.0.15 |
| Zod | 4.5.4 | 4.6.5 |
| react-resizable-panels | 4.12.2 | 4.13.2 |
| tailwind-merge | 3.6.0 | 3.7.0 |
| lucide-react | 1.27.0 | current compatible 1.x |
| Tailwind CSS | 4.3.3 | keep 4.3.x unless a newer compatible release appears |
| `@tailwindcss/vite` | 4.3.3 | keep aligned with Tailwind |

## 2.4 AI Web / assistant UI

| Component | Current | Target |
| --- | ---: | ---: |
| AI SDK `ai` | 7.0.41 | 7.0.113 candidate |
| `@assistant-ui/react` | 0.15.1 | 0.15.22 candidate |
| `@assistant-ui/react-ai-sdk` | 1.4.1 | latest compatible 1.4.x |
| `@assistant-ui/react-data-stream` | 0.12.22 | latest compatible 0.12.x |
| `@assistant-ui/react-markdown` | 0.14.8 | latest compatible 0.14.x |

These packages must be upgraded as a **compatibility set**, not independently.

## 2.5 3D/anatomy frontend

| Component | Current | Target |
| --- | ---: | ---: |
| Three.js | 0.180.0 | 0.186.0 |
| `@react-three/fiber` | 9.6.1 | 9.8.0 |
| `@react-three/drei` | 10.7.7 | 10.7.8 |
| Vanatome React | 0.1.6 | keep unless upstream contract review proves a newer release |
| Vanatome Atlas package | 0.1.4 | keep unless upstream contract review proves a newer release |
| Anatomy atlas data | 1.4.0 | preserve current immutable catalog unless separately promoted |

3D upgrades must preserve:

- mesh/body-region identity;
- picking/hover semantics;
- camera controls;
- lazy loading;
- production chunk budget;
- CDN/static-asset URL contracts.

## 2.6 GraphQL / schema tooling

| Component | Current | Target |
| --- | ---: | ---: |
| Apollo Client | 4.2.12 | 4.3.1 |
| GraphQL | 16.14.2 | 17.0.2 — HOLD while Apollo Server 5.5.1 still requires GraphQL `^16.11.0` |
| Redocly CLI | 2.51.2 | 2.54.2 |
| Orval | 8.30.0 | 8.37.0 |

GraphQL 17 is a major upgrade and must be isolated behind the existing GraphQL lab/tests. It must not be allowed to destabilize the main REST/OpenAPI product path.

## 2.7 Go dependencies

Candidate direct dependency updates discovered during audit:

```text
buf.build/...protovalidate protobuf Go     ...437.1 -> ...437.2
github.com/alicebob/miniredis/v2            2.38.0 -> 2.39.0
github.com/aliyun/alibabacloud-oss-go-sdk-v2 1.5.3 -> 1.6.0
github.com/getkin/kin-openapi                0.142.0 -> 0.149.0
github.com/gin-contrib/requestid             1.0.6 -> 1.0.8
github.com/gin-contrib/slog                  1.2.1 -> 1.2.3
github.com/golang-migrate/migrate/v4         4.19.1 -> 4.20.1
github.com/redis/go-redis/v9                  9.21.0 -> 9.22.0
golang.org/x/crypto                           0.54.0 -> 0.57.0
gorm.io/driver/postgres                       1.6.0 -> 1.6.3
```

Do not run blind `go get -u ./...`; update direct dependencies in bounded groups, then `go mod tidy`.

## 2.8 Python AI dependencies

### Low/medium-risk refresh group

```text
FastAPI                    0.140.13 -> 0.141.1
Uvicorn                    0.51.0   -> 0.53.0
psycopg                    3.3.4    -> 3.3.6
redis-py                   8.0.1    -> 8.1.0
LangGraph                  1.2.10   -> 1.2.12
langgraph-checkpoint-pg    3.1.0    -> 3.1.2
protobuf                   7.36.1   -> 7.36.2
PyMuPDF                    1.28.0   -> 1.28.2
ruff                       0.16.0   -> 0.16.8
pyright                    1.1.411  -> 1.1.414
numpy                      2.5.1    -> 2.5.3
onnxruntime                1.29.0   -> 1.30.0
```

### High-risk AI compatibility group

```text
pydantic-ai-slim           2.31.0 -> 2.49.0
pydantic-evals             2.31.0 -> 2.49.0
openai                     2.50.0 -> 3.19.2
```

This group touches the actual Agent/model abstraction boundary and must be validated against:

- Consultation;
- Diagnosis;
- Treatment;
- Assessment;
- deterministic/TestModel adapters;
- LiteLLM OpenAI-compatible gateway;
- embeddings;
- ASR/Whisper adapter;
- eval capture/tool-call representations.

Do not mix code-level adaptation required by PydanticAI/OpenAI with unrelated formatting/refactors.

## 2.9 Infrastructure/runtime images

| Component | Current | Target |
| --- | ---: | ---: |
| PostgreSQL | pg18 | PG 18.6-compatible pgvector image |
| Redis | 7-alpine | 8.10.2-alpine candidate |
| LiteLLM | 1.97.0 | 1.102.1 candidate |
| nginx | 1.27-alpine | 1.30.5 stable-alpine candidate |
| Caddy | 2-alpine | keep major 2, refresh mirror/digest |
| Node build image | node:24-slim | Node 24.21.x slim + digest |
| Go build image | golang:1.26-alpine | Go 1.26.8 alpine + digest |
| Python AI image | python:3.13-slim digest | Python 3.13.15 slim + digest |

### Redis policy

Redis 8 is a **major runtime upgrade**, even though the current BodySense use is conventional cache/session/rate-limit state.

The upgrade requires:

- real AOF compatibility test;
- persistent-volume backup;
- clean startup using existing data;
- API Redis integration tests;
- session/refresh/rate-limit smoke;
- rollback proof to a pre-upgrade snapshot/volume copy.

No production Redis image switch without that evidence.

---

# 3. Protected contracts

This program must not change these product/runtime contracts merely to facilitate upgrades.

## 3.1 Web/product contracts

Preserve:

- public routes and deep links;
- onboarding/profile/consultation paths;
- authentication/session behavior;
- React Query cache ownership;
- assistant-ui rendering/runtime semantics;
- lazy-loaded Consultation/Diagnosis/3D boundaries;
- BodyExplorer region IDs and selection semantics;
- CSP/CDN/static asset behavior;
- production chunk budgets.

## 3.2 API/data contracts

Preserve:

- OpenAPI contracts and generated artifacts;
- Go ↔ Python runtime protocol;
- SSE/public RuntimeEvent schema;
- PostgreSQL migrations and durable domain data;
- BodyState/Diagnosis/Treatment ownership boundaries;
- Redis key semantics/TTL/session behavior;
- upload/object-storage contracts.

## 3.3 AI contracts

Preserve:

- LiteLLM as the physical provider/routing boundary;
- PydanticAI typed output validation;
- AgentConfiguration identity/governance;
- Diagnosis/Treatment evidence policy;
- deterministic test model behavior;
- qualification/eval semantics unless an upstream API migration requires a strictly equivalent adapter.

## 3.4 Delivery contracts

Preserve:

- exact-SHA candidate artifacts;
- current release lifecycle;
- canonical GCP staging;
- production selection/deploy authority;
- rollback path;
- immutable release evidence.

---

# 4. Non-goals

This program does **not**:

1. redesign the BodySense domain model;
2. change Agent runtime ownership;
3. rewrite the Web application for remote development;
4. replace barrels with deep imports as a performance workaround;
5. move to Node 26;
6. move to pnpm 12;
7. move to Python 3.14;
8. promote PostgreSQL 19 beta;
9. make TypeScript 7 the sole lint/compiler ecosystem before tooling support exists;
10. change Vanatome anatomy content/version without a separate visual/anatomy decision;
11. deploy to production before the full release gate is green.

---

# 5. Execution architecture

The program is one coordinated upgrade, but implementation is divided into containment lanes:

```text
Phase 0  baseline + measurements
    ↓
Phase 1  developer/CI toolchain convergence
    ↓
Phase 2  Vite 8.3 + Bundled Dev remote lane
    ↓
Phase 3  React / runtime Web dependency refresh
    ↓
Phase 4  3D + AI Web compatibility upgrades
    ↓
Phase 5  JS test/schema/build tooling majors
    ↓
Phase 6  Go dependency refresh
    ↓
Phase 7  Python runtime + AI dependency refresh
    ↓
Phase 8  infrastructure images: PG/Redis/LiteLLM/nginx
    ↓
Phase 9  CI actions + immutable image/digest convergence
    ↓
Phase 10 full repository/local-deploy/staging acceptance
    ↓
Phase 11 production release only after explicit acceptance
```

Each phase must leave a diagnosable checkpoint. A failure blocks later phases until repaired or explicitly reverted.

---

# 6. Execution-ready tickets

## BS-UPG-000 — Freeze and characterize the current baseline

**Goal:** capture a reproducible pre-upgrade baseline before any dependency changes.

**Why now:** without a baseline, upgrade failures cannot be attributed reliably.

**Scope:**

- current Git status/HEAD;
- runtime versions;
- lockfile state;
- Web module-request benchmark;
- current focused/full test/build status;
- production chunk sizes;
- current local-deploy gate.

**Out of scope:** dependency changes.

**Protected contracts:** all current contracts.

**Implementation:**

1. Confirm clean worktree and record HEAD.
2. Record `node -v`, `pnpm -v`, `go version`, AI venv Python/uv.
3. Run `pnpm install --frozen-lockfile`.
4. Capture classic Vite request-count benchmark at near-local and simulated 80 ms RTT.
5. Run baseline:
   - `pnpm lint`
   - `pnpm typecheck`
   - `pnpm test`
   - `pnpm build`
   - `pnpm contracts:verify`
   - `git diff --check`
6. Run `pnpm validate:local-deploy` if current host capacity is sufficient.
7. Save evidence in the plan/checkpoint notes.

**Acceptance:** baseline commands and performance numbers are recorded with actual outputs.

---

## BS-UPG-010 — Converge GCP Dev and CI runtime toolchains

**Goal:** make the development host match the runtime lines declared by the repository.

**Scope:**

- Go 1.26.8;
- Node 24 LTS patch refresh;
- pnpm 11.27.1;
- Python 3.13.15 AI environment;
- optional literal Git host upgrade to 2.55.0;
- setup scripts/toolchain docs.

**Implementation:**

1. Upgrade GCP Dev Go from 1.22.2 → 1.26.8.
2. Verify `go env GOTOOLCHAIN`, `go version`, `go test ./...`.
3. Move host Node within 24 LTS to current 24.x patch.
4. Move package manager declaration to pnpm 11.27.1 and align Corepack/setup scripts.
5. Recreate/sync AI venv on Python 3.13.15 without changing feature line.
6. If the earlier “Git version” request was literal, upgrade host Git 2.43.0 → 2.55.0 using a reproducible host-provisioning path; otherwise record Git as non-blocking host drift.
7. Update setup scripts so a new GCP dev host converges to the same versions.

**Acceptance:**

```text
Node 24.x LTS
pnpm 11.27.1
Go 1.26.8
AI Python 3.13.15
```

with repository baseline still green.

**Rollback:** runtime version manager / package pin rollback; no application schema change.

---

## BS-UPG-020 — Upgrade Vite/Nx React plugin foundation

**Goal:** establish the build-tool versions required for the remote-dev fix.

**Scope:**

- Vite 8.3.0;
- `@vitejs/plugin-react` 6.1.1;
- Nx/`@nx/*` 23.2.1;
- compatible esbuild patch.

**Implementation:**

1. Upgrade the four build-tool families together.
2. Regenerate pnpm lockfile only from the intended package changes.
3. Run Web typecheck/build/tests.
4. Run Nx project graph/target discovery.
5. Verify production chunk-budget plugin still executes.

**Acceptance:**

- Web production build green;
- Nx targets resolve;
- chunk budget unchanged;
- no remote-dev behavior change yet.

---

## BS-UPG-021 — Add BodySense Vite Bundled Dev remote lane

**Goal:** collapse the remote browser-facing native ESM waterfall while retaining HMR.

**Reference design:** MemoFlow `apps/web/vite.config.ts` and runtime-lane documentation.

**Scope:**

- `apps/web/vite.config.ts`;
- root development env/default;
- dev scripts/docs;
- Vite config tests;
- optional performance probe script.

**Implementation:**

1. Convert the Vite config to a mode-aware configuration function if required.
2. Load a BodySense-specific dev flag, proposed:
   `BODYSENSE_VITE_BUNDLED_DEV=true`.
3. Enable:
   ```ts
   experimental: {
     bundledDev: useBundledDev,
   }
   ```
   only when:
   - command is `serve`;
   - mode is `development`;
   - flag is true;
   - test lane is not active.
4. Port the MemoFlow Tailwind 4.3.x bundled-dev `hotUpdate` compatibility adapter locally into BodySense Vite config.
5. Keep classic Vite as an explicit fallback:
   `BODYSENSE_VITE_BUNDLED_DEV=false`.
6. Keep Playwright/test lanes on classic Vite.
7. Preserve `@bodysense/contracts` source alias and React dedupe unless actual bundled-dev evidence requires a targeted adjustment.
8. Do **not** rewrite application imports.
9. Add config tests proving:
   - host-dev enables bundled dev;
   - test lane disables it;
   - fallback flag disables it;
   - Tailwind adapter only affects bundled dev.
10. Repeat request-count and latency benchmark.

**Performance acceptance:**

Relative to the captured classic baseline:

- browser-facing source-module request count drops by at least 80%, or equivalent bundled asset evidence demonstrates the same collapse;
- simulated 80 ms RTT page startup is materially improved;
- HMR works for TSX and Tailwind/CSS edits;
- no blank page / unresolved workspace imports;
- fallback classic mode remains operational.

**Rollback:** one env flag disables bundled dev.

---

## Checkpoint evidence — BS-UPG-000 / 010 / 020 / 021 — 2026-09-24

Status: **DONE and independently accepted for the first checkpoint. BS-UPG-030 may proceed; the deferred items below do not block the next implementation phase.** No staging or production deployment was performed.

### DONE

- Repository/toolchain baseline was rechecked on `main`, with Node `v24.21.0`, pnpm `11.27.1`, Go `1.26.8`, uv `0.12.1`, and AI venv Python `3.13.14`.
- `pnpm install --frozen-lockfile` passed. Vite `8.3.0`, `@vitejs/plugin-react` `6.1.1`, Nx/`@nx/*` `23.2.1`, and the exact Rolldown `1.2.6` override are installed from the lockfile.
- Bundled Dev is restricted to `serve` + `development` + `BODYSENSE_VITE_BUNDLED_DEV=true` and is disabled for test/build/fallback lanes. The Tailwind adapter is bounded to bundled mode, while the contracts source alias and React dedupe remain unchanged.
- Focused Vite config tests: **4 passed**. Web TS7 typecheck and production build passed; the `BodyExplorer3D` chunk measured approximately `1,237.25 kB` raw / `286.63 kB` gzip and stayed within its budget.
- Both direct-dev entrypoints inherit the default `BODYSENSE_VITE_BUNDLED_DEV=true`; an explicit `false` override is preserved. `bash -n` passed for the touched dev scripts.
- The classic fallback rendered the login page successfully with `BODYSENSE_VITE_BUNDLED_DEV=false`.

### Final post-pin browser benchmark

The same Playwright login navigation was run against fresh Vite servers in each mode, at 0 ms and 80 ms per-local-request delay. A result is valid only when the login body rendered and no page errors occurred. The single console 401 in bundled mode is the expected unauthenticated API response, not a page error.

| Mode | Delay | Requests | `/src/` | `/node_modules/.vite/` | DCL | Load | Render proof | Page errors |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |
| Bundled | 0 ms | 12 | 0 | 0 | 0.832 s | 0.841 s | login form rendered; body text captured | 0 |
| Bundled | 80 ms | 12 | 0 | 0 | 1.133 s | 1.141 s | login form rendered; body text captured | 0 |
| Classic | 0 ms | 255 | 226 | 22 | 9.771 s | 9.782 s | login form rendered; body text captured | 0 |
| Classic | 80 ms | 255 | 226 | 22 | 13.010 s | 13.019 s | login form rendered; body text captured | 0 |

Bundled Dev therefore reduced browser-facing source-module requests from `226` to `0` in this post-pin apples-to-apples probe and remained render-valid in both latency conditions. Prior pre-pin bundled measurements are intentionally excluded because that lane rendered a blank page.

### Validation evidence

- `pnpm nx run @bodysense/web:typecheck`: **passed**.
- `pnpm nx run @bodysense/web:test`: **passed** as part of the captured `pnpm test` run; the web target completed successfully. A separate single-worker Vitest probe stalled at `BodyStateWorkbench.test.tsx (0 test)` and was discarded as a host/concurrency-sensitive probe, not acceptance evidence.
- `pnpm nx run @bodysense/web:build`: **passed**.
- `pnpm contracts:verify`: **passed**. OpenAPI lint, generated artifacts, breaking/mutation/conformance checks, runtime Proto validation, and Postman route verification all completed successfully.
- `cd apps/api && go test ./...`: **passed** under Go `1.26.8`.
- `git diff --check`: **passed**.

### Independent review acceptance

ChatGPT Web independently reviewed the delegated Codex diff and reran the critical acceptance path after Codex exited successfully:

- `pnpm install --frozen-lockfile`: **passed**.
- Focused Vite config suite: **4/4 passed**.
- Uncached Web TypeScript 7 typecheck: **passed**.
- Uncached Web suite: **53 files / 270 tests passed**; `BodyStateWorkbench.test.tsx` completed normally, confirming the earlier single-worker timeout was probe-specific rather than a regression.
- Uncached Web production build: **passed**; `BodyExplorer3D` remained approximately `1,237.25 kB` raw / `286.63 kB` gzip.
- `pnpm contracts:verify`: **passed**.
- `cd apps/api && go test -count=1 ./...`: **passed** under Go `1.26.8`.
- `git diff --check`: **passed**; benchmark probe edits were not present in `LoginPage.tsx` or `index.css`.

### Non-blocking holds / deferred gates

- AI venv remains Python `3.13.14`: uv has no downloadable `3.13.15` build on this host. Do not compile Python from source solely to chase the patch number; this is a non-blocking toolchain hold, not a checkpoint regression.
- Host Git remains Ubuntu `2.43.0`. No third-party Git repository was added; this remains non-blocking host drift.
- Full repository `pnpm lint`, `pnpm typecheck`, and `pnpm test`: **passed** in the delegated implementation run. `pnpm validate:local-deploy` was intentionally not run because this checkpoint excludes deployment-like environment mutation; it remains a required gate before staging/release promotion, not before BS-UPG-030.

## BS-UPG-030 — Refresh React and common Web runtime dependencies

**Goal:** move the ordinary React application stack to current compatible versions before touching 3D and AI-specific surfaces.

**Scope:**

- React/React DOM 19.3;
- React Router 8.4;
- TanStack Query 5.103;
- Base UI 1.8;
- Zustand/Zod/panels/icons/tailwind-merge;
- React type packages.

**Implementation:**

1. Upgrade React + React DOM + types as one set.
2. Upgrade Router/Query/state/schema/UI primitives.
3. Resolve only real API/type changes; avoid style/refactor churn.
4. Run focused:
   - routing/auth;
   - profile/onboarding;
   - consultation shell;
   - QueryClient-related tests.
5. Run Web full suite + typecheck + build.

**Acceptance:** no product-path behavior regression and no new React runtime warnings.

### BS-UPG-030 acceptance evidence — 2026-09-25

Status: **DONE / ACCEPTED.**

- Upgraded React/React DOM to `19.3.0`, React Router to `8.4.0`, TanStack Query to `5.103.2`, Base UI to `1.8.0`, Zustand to `5.0.15`, Web Zod to `4.6.5`, react-resizable-panels to `4.13.2`, lucide-react to `1.47.0`, tailwind-merge to `3.7.0`, sonner to `2.0.8`, and React type packages to `19.3.0`.
- Pulled `@react-three/fiber` forward from `9.6.1` to `9.8.0` as a compatibility prerequisite: Fiber `9.6.1` declares React/React DOM `>=19 <19.3`, while Fiber `9.8.0` declares `>=19 <19.4`. `pnpm peers check` is clean after the bump.
- Kept the root contract-toolchain Zod pin at exactly `4.5.4`; only the Web runtime uses `4.6.5`. This preserves `scripts/contracts/check-toolchain.mjs` and generated-contract reproducibility.
- Removed `next-themes 0.4.6`. BodySense already forced `light` and disabled system theme, while React 19.3 reports client-rendered `<script>` elements emitted by `next-themes` as a runtime console error. The equivalent current product contract is now explicit: `:root { color-scheme: light; }`, and the Sonner toaster uses `theme="light"` directly.
- Focused regression suite after the theme simplification: **6 files / 17 tests passed**.
- Uncached Web lint: **passed**.
- Uncached TypeScript 7 Web typecheck: **passed**.
- Uncached Web full suite: **53 files / 270 tests passed**.
- Uncached production build: **passed**. `BodyExplorer3D` is approximately `1,253.52 kB` raw / `291.28 kB` gzip, still within the explicit `1,300 kB` raw / `300 kB` gzip budget.
- `pnpm contracts:verify`: **passed**.
- `pnpm peers check`: **no peer dependency issues**.
- Bundled Dev Chromium smoke: login rendered, `#root script` count `0`, computed `color-scheme: light`, `pageerror=0`, React runtime issues `0`.
- Production preview Chromium smoke: same result — login rendered, `#root script` count `0`, `color-scheme: light`, `pageerror=0`, React runtime issues `0`. The isolated smoke intentionally had no API backend, so the auth refresh request returned the expected proxy/resource error and was not treated as an application regression.

---

## BS-UPG-040 — Qualify remaining Three.js / R3F / Drei compatibility set

**Goal:** modernize the 3D renderer without changing anatomy semantics or violating Vanatome peer contracts.

**Current compatibility boundary:**

- `@react-three/fiber 9.8.0` remains the accepted local version; React 19.3 requires the 9.8 line, while the newer 9.8.1 patch is intentionally held by the repository minimum-release-age policy.
- `@react-three/drei 10.7.8` is the accepted local version.
- `three` remains on `0.180.0`. Current `@vixotic/vanatome-react 0.1.6` declares `three ^0.180.0`, which does **not** admit Three `0.186.x`; therefore the previous unconditional Three `0.186` target is withdrawn until Vanatome expands its peer range or an explicit compatibility qualification justifies an override.

**Implementation:**

1. Recheck the current Vanatome peer contract before changing Three.
2. Keep Fiber at `9.8.0` while the newer patch is inside the minimum-release-age window; upgrade Drei to `10.7.8` if the peer graph remains clean.
3. Do not move Three beyond `0.180.x` while Vanatome still requires `^0.180.0` unless a deliberate compatibility exception is designed and validated.
4. Run BodyExplorer unit/component tests.
5. Run production build and compare `BodyExplorer3D` chunk budget.
6. Exercise:
   - model load;
   - region hover/select;
   - camera orbit/reset;
   - side/front/back interaction if present;
   - error/fallback state.
7. Verify anatomy catalog/CDN URLs unchanged.
8. Perform a visual smoke on staging after integration.

**Acceptance:** peer graph remains clean, semantic region selection remains identical, and the lazy chunk remains inside the explicit budget.

### BS-UPG-040 local acceptance evidence — 2026-09-25

Status: **LOCAL DONE / ACCEPTED; staging visual pointer-hit gate deferred to promotion.**

- Kept `@react-three/fiber` at `9.8.0` and upgraded `@react-three/drei` from `10.7.7` to `10.7.8`. A trial of Fiber `9.8.1` was rejected by the repository minimum-release-age gate together with its fresh `its-fine 2.1.1` transitive dependency, so no supply-chain exception was added.
- Kept `three` at `0.180.0` because the current latest `@vixotic/vanatome-react 0.1.6` still declares `three ^0.180.0`; moving to the registry latest Three `0.186.x` would violate the peer contract.
- `pnpm peers check`: **no peer dependency issues**.
- BodyExplorer focused suite: **9 files / 33 tests passed**.
- Uncached TypeScript 7 Web typecheck: **passed**.
- Uncached full Web suite: **53 files / 270 tests passed**.
- Uncached production build: **passed**. With the accepted Fiber `9.8.0` / Drei `10.7.8` combination, `BodyExplorer3D` is approximately `1,253.52 kB` raw / `291.29 kB` gzip, inside the explicit `1,300 kB` raw / `300 kB` gzip budget.
- Anatomy catalog/CDN configuration and pinned atlas release/build were unchanged. The BodySense R2 catalog `https://assets.bakersean.top/anatomy/vanatome/1.4.0/releases/1.4.0/catalog.json` returned HTTP 200 with the expected atlas `1.4.0` / build `994e6cc8ffbb212e` during validation.
- A temporary browser-only smoke harness mounted the real `BodyExplorer3D` against the self-hosted R2 atlas. Chromium/SwiftShader verified the regional model reached `ready`, a real WebGL context existed, the skeletal system loaded on demand and reached `ready`, controlled durable anatomy selection for the left clavicle reconciled through the viewer, focus/isolation/X-Ray/canvas-orbit/reset flows completed, and there were **0 page errors, 0 unexpected console issues, 0 fatal viewer errors, and 0 non-API failed requests**.
- Direct headless pointer hit-testing against a regional structure was not treated as acceptance evidence because deterministic mesh-pixel selection was not reliable under SwiftShader. The semantic selection contract remains covered by the BodyExplorer/adapter tests; an actual pointer hover/select visual check remains a staging promotion gate rather than a blocker for BS-UPG-041.
- The temporary smoke harness and port were removed before acceptance; no smoke-only source files remain.

**Rollback:** revert only the remaining 3D package changes; no data migration.

---

## BS-UPG-041 — Converge assistant-ui runtime without changing the BodySense event contract

**Goal:** modernize the browser AI rendering stack while preserving BodySense's existing `useLocalRuntime` + Go SSE/durable replay architecture.

**Architecture finding:**

- BodySense application source directly uses `@assistant-ui/react` and `@assistant-ui/react-markdown`.
- The previous direct dependencies `ai`, `@assistant-ui/react-ai-sdk`, and `@assistant-ui/react-data-stream` had **zero source imports** in the Web application. The actual model/runtime integration is BodySense's custom `ChatModelAdapter` backed by the Go event stream, not the Vercel AI SDK adapter.
- Therefore the correct convergence is to remove those unused direct integration dependencies rather than upgrade them and accidentally imply a runtime ownership change.

**Implementation:**

1. Remove unused direct dependencies `ai`, `@assistant-ui/react-ai-sdk`, and `@assistant-ui/react-data-stream`.
2. Upgrade only the assistant-ui packages that BodySense actually imports.
3. Preserve the custom `useLocalRuntime` / `ChatModelAdapter` / SSE reducer / durable replay contract.
4. Keep the repository minimum-release-age and production chunk budgets as hard gates; do not add supply-chain exclusions or raise budgets just to accept newer package versions.
5. Pin a coherent assistant-ui internal release family when broad upstream caret ranges would otherwise mix export-incompatible package generations.
6. Run focused Consultation coverage for streaming text, tool/HITL rendering, disconnect/replay, history projection, and the workbench shell.
7. Verify no RuntimeEvent schema change is introduced merely for library compatibility.

### BS-UPG-041 local acceptance evidence — 2026-09-25

Status: **LOCAL DONE / ACCEPTED.**

- Removed unused direct dependencies `ai`, `@assistant-ui/react-ai-sdk`, and `@assistant-ui/react-data-stream`; repository source has no direct imports of those integrations.
- Accepted `@assistant-ui/react 0.15.17` and `@assistant-ui/react-markdown 0.14.16`.
- Pinned the coherent internal runtime family required by assistant-ui `0.15.17`: `@assistant-ui/core 0.3.16`, `@assistant-ui/store 0.3.11`, `@assistant-ui/tap 0.9.15`, `assistant-cloud 0.1.42`, `assistant-stream 0.3.40`, and `safe-content-frame 0.0.28`. Without this pin, broad upstream caret ranges can combine newer core packages with `assistant-cloud 0.1.x` and fail production resolution on the missing `assistant-cloud/ai-sdk` export.
- Newer direct candidates were deliberately rejected rather than weakening repository policy: `0.15.22` / markdown `0.14.17` were inside the active minimum-release-age window, while assistant-ui `0.15.19`–`0.15.21` produced an approximately `529 kB` raw `AssistantChatPanel` chunk and failed the existing `500,000` byte production chunk budget.
- The accepted `0.15.17` combination builds `AssistantChatPanel` at approximately `486.70 kB` raw / `143.24 kB` gzip, preserving the production budget without raising it.
- `pnpm install --frozen-lockfile`: **passed** with no new minimum-release-age exclusions.
- `pnpm peers check`: **no peer dependency issues**.
- Uncached TypeScript 7 Web typecheck: **passed**.
- Focused Consultation compatibility suite: **15 files / 140 tests passed** on the final candidate, covering streaming, tool/HITL projections, active-turn state, SSE validation, durable recovery, thread mapping, conversation actions, workbench shell, and Consultation page behavior.
- Uncached full Web suite: **53 files / 270 tests passed**.
- `pnpm contracts:verify`: **passed**; the public RuntimeEvent/OpenAPI/Proto/Postman contracts remain unchanged.
- `scripts/validate-supply-chain.sh`: **passed**, `high=0`, `critical=0`.
- Production build: **passed** with the existing chunk budgets; no budget was relaxed for this upgrade.

**Acceptance:** the current Go public event stream drives equivalent UI state after the upgrade, no RuntimeEvent contract changed, supply-chain policy remains intact, and the production chat chunk remains within the pre-existing hard budget.

---

## BS-UPG-050 — Upgrade JS test/build/schema tooling majors

**Goal:** modernize development-only tooling after runtime application code is stable.

**Scope:**

- Vitest 5;
- Playwright 1.63;
- happy-dom;
- Testing Library;
- jest-dom 7 if compatibility is proven;
- ESLint 10.11;
- typescript-eslint 8.70;
- Prettier;
- Redocly;
- Orval;
- Apollo 4.3 / GraphQL 17 as an isolated sub-ticket.

**Implementation:**

1. Upgrade lint/format patch/minors first.
2. Upgrade Vitest 5 and fix config/API changes.
3. Upgrade Playwright and browser binaries.
4. Upgrade test DOM/testing-library packages.
5. Upgrade Redocly/Orval and run generated-contract parity.
6. Upgrade Apollo Client + GraphQL 17 only inside the GraphQL lab boundary.
7. Preserve TypeScript dual-lane policy.
8. Run all tests and contract generation/check-generated gates.

**Acceptance:** test count does not silently shrink; generated contract diff is either empty or explicitly reviewed.

### BS-UPG-050 local acceptance evidence — 2026-09-25

Status: **COMPATIBLE SUBSET DONE / ACCEPTED; Vitest 5 and GraphQL 17 remain explicit ecosystem holds.**

Accepted upgrades:

- ESLint `10.8.0` -> `10.11.0`;
- typescript-eslint `8.65.0` -> `8.70.1`;
- Prettier `3.9.6` -> `3.9.9`;
- Playwright `1.62.1` -> `1.63.0`, with the matching Chromium browser installed on GCP Dev;
- happy-dom `20.11.1` -> `20.14.5`;
- `@testing-library/react` `16.3.2` -> `16.3.3`;
- `@testing-library/user-event` `14.6.1` -> `14.6.7`;
- `@testing-library/jest-dom` `6.9.1` -> `7.0.1`;
- Redocly CLI `2.51.2` -> `2.54.2`;
- Orval `8.30.0` -> `8.37.0`;
- Apollo Client `4.2.12` -> `4.3.1` inside the isolated GraphQL learning lab.

Explicit holds:

- **Vitest 5.0.1:** current latest Nx `23.2.1` / `@nx/vitest 23.2.1` declares Vitest peer support only for `^3 || ^4`. The trial was reverted; the repository resolves Vitest `4.1.11` with a clean peer graph. Do not ignore this peer or add an override merely to claim Vitest 5 support.
- **GraphQL 17.0.2:** Apollo Client `4.3.1` accepts GraphQL 16/17, but current Apollo Server `5.5.1` still declares GraphQL `^16.11.0`. GraphQL therefore remains `16.14.2` until Apollo Server expands its peer range or a newer server release is deliberately qualified.

Validation evidence:

- `pnpm lint`: **5 projects passed**.
- `pnpm typecheck`: **3 projects passed**.
- `pnpm build`: **Web + API passed** with existing production chunk budgets intact.
- Web unit/component suite under Vitest `4.1.11`, happy-dom `20.14.5`, and the upgraded Testing Library stack: **53 files / 270 tests passed**.
- Uncached repository test gate: **Contracts + Web + API + AI Service all passed**, `33.6 s`, with Nx cache disabled.
- Playwright `1.63.0` successfully discovers the existing **10 Chromium E2E tests in 6 files**; full environment-dependent execution remains part of later convergence/staging gates.
- GraphQL Part 8 learning lab under Apollo Client `4.3.1`: **4/4 tests passed** with Apollo Server `5.5.1` + GraphQL `16.14.2`.
- Canonical contract-tool pins were updated to Redocly `2.54.2` / Orval `8.37.0`; `pnpm contracts:verify` passed and regeneration produced **zero generated Git diff**.
- `pnpm install --frozen-lockfile`: **passed**.
- `pnpm peers check`: **no peer dependency issues**.
- `scripts/validate-supply-chain.sh`: **passed**, `high=0`, `critical=0`.
- Selected configuration/manifests pass Prettier `3.9.9`; `git diff --check` passes.

The two held majors are compatibility constraints, not acceptance failures. They should be revisited when Nx and Apollo Server publish compatible peer ranges rather than bypassed locally.

---

## BS-UPG-060 — Refresh Go dependencies on Go 1.26.8

**Goal:** upgrade direct Go dependencies under the actual target Go toolchain.

**Implementation order:**

1. security/stdlib-adjacent:
   - `golang.org/x/crypto`;
2. persistence/client:
   - `go-redis/v9`;
   - GORM postgres;
   - OSS SDK;
3. HTTP/schema:
   - kin-openapi;
   - gin-contrib;
   - protovalidate/protobuf;
4. tooling/test:
   - migrate;
   - miniredis.
5. `go mod tidy`.
6. Run:
   ```bash
   cd apps/api
   go test ./...
   go test -race ./internal/...   # scoped as feasible
   go vet ./...
   ```
7. Run contract and migration replay gates.

**Acceptance:** no OpenAPI/runtime/storage behavior drift.

### BS-UPG-060 local acceptance evidence — 2026-09-25

Status: **LOCAL DONE / ACCEPTED.**

Direct Go modules upgraded under host Go `1.26.8`:

- generated Protovalidate protobuf Go package `...437.1` -> `...437.2`;
- `github.com/alicebob/miniredis/v2` `2.38.0` -> `2.39.0`;
- `github.com/aliyun/alibabacloud-oss-go-sdk-v2` `1.5.3` -> `1.6.0`;
- `github.com/getkin/kin-openapi` `0.142.0` -> `0.149.0`;
- `github.com/gin-contrib/requestid` `1.0.6` -> `1.0.8`;
- `github.com/gin-contrib/slog` `1.2.1` -> `1.2.3`;
- `github.com/golang-migrate/migrate/v4` `4.19.1` -> `4.20.1`;
- `github.com/redis/go-redis/v9` `9.21.0` -> `9.22.0`;
- `golang.org/x/crypto` `0.54.0` -> `0.57.0`;
- `gorm.io/driver/postgres` `1.6.0` -> `1.6.3`.

`go mod tidy` also converged compatible indirects including pgx `5.10.0`, jsonschema/v6 `6.0.3`, x/net `0.58.0`, x/sync `0.23.0`, x/sys `0.48.0`, x/text `0.42.0`, x/time `0.15.0`, and migrate-related Docker/OpenTelemetry dependencies. A final `go list -m -u` showed no remaining direct-module updates.

Validation evidence:

- `go mod verify`: **all modules verified**.
- `go test -count=1 ./...`: **passed** across all API packages; Redis/miniredis auth/cache paths and HTTP transport tests completed under the new modules.
- `go vet ./...`: **passed**.
- `go test -count=1 -race ./internal/...`: **passed**; the HTTP API package completed in approximately `47 s` and no race was reported.
- Disposable PostgreSQL 18 migration replay using `scripts/schema/capture-current.sh` with output redirected to `/tmp`: **passed** — `FULL_UP=PASS`, `LATEST_DOWN=PASS`, `LATEST_REPLAY_UP=PASS`; final snapshot `1:false`, `48` tables, `672` columns.
- `pnpm contracts:verify`: **passed** after the Go upgrades; generated contract artifacts remained clean.
- `git diff --check`: **passed**.

No application source code, OpenAPI schema, migration, or generated contract file required modification; this checkpoint is dependency/toolchain-only.

---

## BS-UPG-070 — Patch-refresh Python runtime and low/medium-risk dependencies

**Goal:** establish the new Python 3.13.15 baseline before changing Agent library APIs.

**Implementation:**

1. Refresh Python image to 3.13.15 slim and new immutable digest.
2. Upgrade low/medium-risk Python direct dependencies.
3. Regenerate `uv.lock`.
4. Run:
   - Ruff;
   - Pyright;
   - Python full tests;
   - OCR focused tests;
   - document extraction smoke;
   - RAG/Postgres integration where available.
5. Build AI runtime-base image and verify native packages/models.

**Acceptance:** no Agent-library major/minor API migration yet; pure runtime/dependency refresh is green.

### BS-UPG-070 local acceptance evidence — 2026-09-25

Status: **LOCAL DONE / ACCEPTED.**

Runtime/dependency changes:

- Refreshed the AI runtime base to explicit `python:3.13.15-slim@sha256:8d9d0b8bcf6506481eae4907c18f5e3e7902e629f5f6d684f9e7c32e85e3ddf0` for both dependency-builder and runtime-base stages.
- Upgraded FastAPI `0.140.13` -> `0.141.1`, Uvicorn `0.51.0` -> `0.53.0`, psycopg `3.3.4` -> `3.3.6`, redis-py `7.0.0` -> `8.1.0`, python-dotenv `1.2.2` -> `1.2.3`, LangGraph `1.2.10` -> `1.2.12`, LangGraph PostgreSQL checkpoint `3.1.0` -> `3.1.2`, protobuf `6.32.1` -> `7.36.2`, numpy `2.5.0` -> `2.5.3`, httpx2 `2.9.0` -> `2.13.1`, Ruff `0.16.0` -> `0.16.9`, and Pyright `1.1.411` -> `1.1.414`.
- Kept OpenAI `2.50.0`, `pydantic-ai-slim 2.31.0`, `pydantic-evals 2.31.0`, sentence-transformers `5.6.1`, ONNXRuntime `1.29.0`, and RapidOCR `3.9.2` unchanged so BS-UPG-071/072 and native/ML major boundaries remain isolated.
- Tightened already-qualified mechanism dependencies to exact pins: PyMuPDF `==1.28.0` for `health-document-v20` and MediaPipe `==1.0.0` for `posture-v2`. Both versions are part of fail-closed mechanism identity and must not drift as ordinary patch upgrades.
- GCP Dev's uv-managed development interpreter remains Python `3.13.14` because uv does not currently provide a downloadable `3.13.15` build on this host. This is non-blocking: the immutable runtime/application images are verified on Python `3.13.15`; no source-built host Python was introduced solely to chase a patch number.

Pre-existing mechanism identity repair discovered during runtime smoke:

- `health-document-v20` declared `admissibility_policy_sha256=5d67fa3d...`, while current HEAD had drifted to `08226b14...` after the legacy-runtime cleanup removed only the backward-compatible V1 symbol `OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION`.
- Git history showed the manifest-bound source came from the original governed pipeline commit and the later diff removed only that alias, not V2 policy behavior. Restoring the two compatibility lines returns the source file **exactly** to the manifest-bound SHA-256 `5d67fa3d0dfa96a915c8392db6e0004be2b87b508df106a2eb17a84fd4eda79b` without changing the health-document configuration id or policy semantics.
- Added a regression test asserting the verified manifest's admissibility-policy source identity matches the current source so similar source/manifest drift fails during unit tests instead of first appearing in a worker process.

Validation evidence:

- `uv sync --frozen --extra dev --extra ocr --extra pose --extra document-ocr`: **passed**.
- Ruff: **passed**.
- Pyright: **0 errors / 0 warnings**.
- Uncached AI-service suite: **508 tests passed** under the GCP Dev Python `3.13.14` interpreter.
- Real host-level document-service smoke generated a born-digital PDF and exercised `POST /api/ocr/extract`: **passed**, retaining configuration `hdex-config-f2495c95b6ed9de2`, PyMuPDF `1.28.0`, ONNXRuntime `1.29.0`, and `native_pdf_text` routing.
- Real pose smoke provisioned the pinned model and exercised MediaPipe Tasks geometry: **passed**, retaining `posture-config-efa3a84622818772`, `posture-geometry-v1`, and the pinned model SHA-256 `59929e1d...`.
- Disposable PostgreSQL `18.6` + pgvector `0.8.6` integration: **passed**. `AsyncPostgresSaver.setup()` created the checkpoint tables, an actual checkpoint was written/read successfully, KnowledgeLibrary's psycopg pool registered pgvector, and a vector query succeeded.
- `runtime-base` image `bodysense-ai-runtime-base:b070` built successfully with Python `3.13.15`, Tesseract `5.5.0`, the pinned pose model, the health-document model bundle, and the accepted Python package set.
- Final application image `bodysense-ai-service:b070` built successfully from that runtime base. A temporary main AI container connected to the disposable PG18 instance and returned `GET /health` **200** after the full checkpointer + KnowledgeLibrary lifespan initialized. The same image, running `src.document_main:app`, returned document-service health **200**.
- Container-level `POST /api/ocr/extract` against the final application image: **passed** with the same `hdex-config-f2495c95b6ed9de2` provenance and native-PDF extraction path.
- `uv lock --check`: **passed**.
- `pnpm contracts:verify`: **passed**; generated contract artifacts remained clean.
- `scripts/validate-supply-chain.sh`: **passed**, `high=0`, `critical=0`.
- `git diff --check`: **passed**.
- All disposable BS-UPG-070 PostgreSQL/AI/document smoke containers and temporary listener ports were removed after validation. No staging or production service was mutated.

---

## BS-UPG-071 — Upgrade PydanticAI/Pydantic Evals

**Goal:** move PydanticAI/Pydantic Evals from `2.31.0` to the current compatible `2.50.0` line while preserving typed Agent semantics.

**Implementation:**

1. Upgrade `pydantic-ai-slim`, `pydantic-evals`, associated graph/core packages coherently.
2. Compile/typecheck before code edits.
3. Adapt:
   - Model abstraction;
   - Agent generics;
   - `RunContext`;
   - tool-call/message parts;
   - TestModel/FunctionModel;
   - eval capture.
4. Run focused agent tests:
   - Consultation;
   - Diagnosis;
   - Treatment;
   - Assessment;
   - deterministic adapters;
   - qualification/evals.
5. Verify LiteLLM OpenAI-compatible gateway remains the provider boundary.

**Acceptance:** typed output schemas and tool-call behavior remain equivalent; qualification gates do not degrade.

**Dependency boundary discovered during implementation:** `pydantic-ai-slim[openai] 2.31.0` is the last release that still accepts OpenAI 2.x. Starting at `2.32.0`, the OpenAI extra requires OpenAI 3.x (`>=3.0.0`), and the current `2.50.0` release requires OpenAI `>=3.19.0`. Therefore BS-UPG-071 cannot produce a valid standalone intermediate state while keeping BS-UPG-072 deferred. The two tickets are executed and accepted atomically as one compatibility checkpoint.

---

## BS-UPG-072 — Upgrade OpenAI Python SDK 2.x → 3.x

**Goal:** modernize direct OpenAI-compatible client use without bypassing LiteLLM architecture.

**Affected areas include:**

- AI service direct client;
- RAG embeddings;
- knowledge-library error handling;
- ASR/Whisper API adapter.

**Implementation:**

1. Upgrade OpenAI atomically with the PydanticAI/Pydantic Evals ticket because the accepted PydanticAI line requires OpenAI 3.x.
2. Fix explicit API/type changes only.
3. Verify custom `base_url` + API-key behavior against LiteLLM.
4. Run embedding and ASR mocks/focused integration.
5. Verify retry/error normalization.
6. Run Python full suite.

**Acceptance:** no direct provider-routing regression and no accidental bypass of LiteLLM.

### BS-UPG-071/072 atomic local acceptance evidence — 2026-09-25

Status: **LOCAL DONE / ACCEPTED AS ONE ATOMIC COMPATIBILITY CHECKPOINT.**

Final coherent package set:

- `pydantic-ai-slim[openai]`: `2.31.0` -> `2.50.0`;
- `pydantic-evals`: `2.31.0` -> `2.50.0`;
- `pydantic-graph`: `2.31.0` -> `2.50.0`;
- `genai-prices`: `0.1.3` -> `0.1.9`;
- `openai`: `2.50.0` -> `3.19.2`.

Compatibility findings:

- A deliberate attempt to hold OpenAI at `2.50.0` while upgrading PydanticAI/Evals to `2.50.0` failed dependency resolution, as expected: PydanticAI `2.50.0` requires OpenAI `>=3.19.0` for its OpenAI extra.
- Registry metadata confirmed the exact transition boundary: PydanticAI `2.31.0` permits OpenAI `>=2.45.0`; PydanticAI `2.32.0` through `2.38.0` require OpenAI `>=3.0.0`; later releases increase that floor, with `2.50.0` requiring `>=3.19.0`.
- The plan therefore treats 071/072 as one atomic runtime-compatibility upgrade instead of manufacturing an invalid intermediate state.

Validation evidence:

- Uncached Pyright after the atomic upgrade: **0 errors / 0 warnings**; no Agent source adaptation was required.
- Focused Consultation/Diagnosis/Treatment/Assessment/eval compatibility suite: **68 tests passed**, covering Agent generics, `RunContext`, typed outputs, TestModel/FunctionModel behavior, tool-call/message parts, qualification and eval capture.
- Focused direct OpenAI/LiteLLM/RAG/ASR suite: **45 tests passed**, covering embeddings, ASR adapter behavior, gateway routing, LiteLLM configuration, KnowledgeLibrary async behavior and attribution paths.
- Full uncached AI-service suite: **508 tests passed**.
- Ruff: **passed**.
- Local OpenAI-compatible HTTP stub validation: BodySense `AIService` using OpenAI `3.19.2` successfully completed both non-streaming and SSE streaming `/v1/chat/completions` calls through a custom `base_url`; token usage and finish reason mapping remained correct.
- The same stub also validated PydanticAI `2.50.0` `OpenAIChatModel` through `OpenAIProvider(base_url=..., api_key=...)`, confirming the LiteLLM-compatible provider boundary remains intact.
- Repeated the provider smoke **inside the final Python 3.13.15 application image**: direct non-streaming, streaming, and PydanticAI model calls all passed.
- Rebuilt `bodysense-ai-runtime-base:b07172` on Python `3.13.15`; image inspection confirmed PydanticAI/Evals/Graph `2.50.0`, OpenAI `3.19.2`, while mechanism-bound PyMuPDF `1.28.0`, MediaPipe `1.0.0`, ONNXRuntime `1.29.0`, and RapidOCR `3.9.2` remained unchanged.
- Built final application image `bodysense-ai-service:b07172` from that runtime base. With a disposable PostgreSQL `18.6` + pgvector instance, the full FastAPI lifespan initialized and `GET /health` returned **200**.
- `uv lock --check`: **passed**.
- `pnpm contracts:verify`: **passed**; generated contract artifacts remained clean.
- `scripts/validate-supply-chain.sh`: **passed**, `high=0`, `critical=0`.
- `git diff --check`: **passed**.
- All temporary OpenAI stub, PostgreSQL, and application-smoke processes/containers/ports were removed after validation. No staging or production environment was changed.

No BodySense Agent/runtime source code required modification for this upgrade; the accepted diff is dependency/lockfile plus plan evidence only.

---

## BS-UPG-080 — Upgrade PostgreSQL 18 patch image and validate migrations

**Goal:** keep current PG18 architecture while moving to a current security/bugfix patch.

**Implementation:**

1. Resolve the current pgvector PG18 image digest corresponding to a PG 18.6-compatible base.
2. Update dev/staging/prod mirrors consistently.
3. Run fresh DB:
   `000001 → latest`.
4. Run latest down/up replay.
5. Restore a representative backup into the new image.
6. Run API integration/local deploy validation.

**Acceptance:** schema/migration replay and restore are green; no major-version migration.

### BS-UPG-080 local acceptance evidence — 2026-09-25

Status: **LOCAL DONE / ACCEPTED.**

Infrastructure convergence:

- Resolved the current `pgvector/pgvector:pg18` image to immutable digest `sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a`.
- Verified that this digest runs PostgreSQL `18.6` with pgvector `0.8.6`, and that the existing dev/staging runtime plus the production mirror source already correspond to this same runtime lineage.
- Replaced floating `pgvector/pgvector:pg18` references in dev/staging Compose, CI service containers, schema capture, deploy preflight, DR/privacy/lease validators, and PostgreSQL client wrappers with the immutable digest. Production Compose continues to consume the controlled ACR mirror; its source mirror contract remains digest-bound.
- The repository's PostgreSQL 18 production contract was tightened so CI must use the same immutable digest rather than merely the `pg18` tag.

Database validation evidence:

- Fresh disposable pinned PG18 migration validation: **passed** — `FULL_UP=PASS`, `LATEST_DOWN=PASS`, `LATEST_REPLAY_UP=PASS`.
- `scripts/schema/capture-current.sh` against the pinned digest: **passed**; snapshot metadata records the immutable image, migration state `1:false`, `48` tables, and `672` columns for the fresh schema.
- Current staging was backed up read-only with `pg_dump -Fc` and restored into a second disposable pinned PG18 instance: **passed**.
- Restored runtime identity: PostgreSQL `18.6` + pgvector `0.8.6`.
- Staging-vs-restored public table set: **52/52 exact match**.
- Staging-vs-restored exact per-table row counts: **52/52 exact match**.
- `scripts/validate-production-postgres18.sh`: **passed**.
- `scripts/validate-migration-history.sh`: **passed**.

Runtime/local-deploy evidence:

- The disposable production-shaped stack successfully built and started PostgreSQL, Redis, LiteLLM, API, AI Service, Document Service, and Web.
- Health gates: `API_HEALTH=PASS`, `AI_HEALTH=PASS`, `WEB_HEALTH=PASS`.
- Runtime/domain gates passed: posture geometry mechanism, migration replay, BodyState semantics, BodyRegion ID roundtrip, Treatment activation atomicity, Outcome feedback atomicity, domain semantics, knowledge legacy-publish disablement, operator gate, publication vertical, and rollback vertical.
- Playwright on the disposable stack: **10/10 Chromium E2E tests passed**, including Body Explorer, Consultation runtime recovery, longitudinal health/full-loop, assessment evidence, and lifestyle current-state update.
- Diagnosis/Treatment baseline, decision-trace, and replay-input validators all passed.
- The validator reached `LOCAL_DEPLOY_VALIDATION=PASS`. Its pre-fix shell exit was nonzero only because `docker buildx rm -f` reported a transient builder-removal failure after the builder had in fact disappeared.

Engineering fixes discovered while closing the gate:

- Both Web Docker build paths now default to the canonical `https://registry.npmjs.org` for authoritative minimum-release-age metadata; China-local builds may still explicitly override `NPM_REGISTRY=https://registry.npmmirror.com`. This fixed a real container-build failure where the mirror had not yet published metadata for `@orval/zod@8.37.0`.
- Disposable/dev Compose now defaults `BODYSENSE_VITE_BUNDLED_DEV=true`. Playwright tracing proved that classic Vite's cold dependency-optimizer restart emitted `net::ERR_NETWORK_CHANGED` during lazy Body Explorer/Assistant imports; Bundled Dev removed that failure and the previously failing Body Explorer E2E passed.
- Delivery contract expectations were updated to the accepted immutable Python `3.13.15-slim@sha256:...` base introduced in BS-UPG-070; delivery suite remains **43/43 passed**.
- Validator teardown now uses bounded builder-removal retries plus postcondition verification. A nonzero `buildx rm` is accepted only when `buildx ls` proves the builder is gone; a still-existing builder remains a hard failure.
- Validator lifecycle regression suite: **5/5 passed**, covering normal cleanup, recovered transient removal, bounded persistent failure, keep-stack behavior, and isolated local-deploy ownership.
- Real Buildx cleanup smoke: **passed**; isolated builder created, removed, and verified absent.

Final gates:

- `pnpm contracts:verify`: **passed**, generated contract artifacts clean.
- `scripts/validate-supply-chain.sh`: **passed**, `high=0`, `critical=0`.
- `git diff --check`: **passed**.
- No validator builders, containers, networks, or volumes remain after validation.
- No live staging or production service was rebuilt or deployed during BS-UPG-080; staging access was read-only for the representative backup/restore qualification.

---

## BS-UPG-081 — Upgrade Redis 7 → Redis 8.10.2

**Goal:** modernize Redis with a reversible persistent-data migration.

**Implementation:**

1. Create an AOF/data-volume backup before first Redis 8 start.
2. Upgrade dev first.
3. Start Redis 8 against a copied dev volume.
4. Verify AOF load/rewrite.
5. Run:
   - session/auth refresh;
   - rate limiting;
   - JobRuntime/locks if Redis-backed;
   - API Redis package tests.
6. Repeat on staging with backup + rollback drill.
7. Only then update production mirrored image/digest.

**Acceptance:**

- existing data loads;
- no key/TTL semantic regression;
- auth/session flow works;
- rollback to pre-upgrade snapshot is demonstrated.

**Local acceptance evidence (2026-09-25):**

- The disposable `bodysense-b081-runtime-108113` Redis 8.10.2 runtime loaded copied dev and staging data with AOF enabled; `BGREWRITEAOF` and `PING` succeeded. Original dev/staging Redis 7 services and volumes were not mutated.
- Canonical logical snapshots matched across Redis 7 and Redis 8: dev 52/52 keys and TTLs (26 sets, 26 strings); staging 99/99 keys and TTLs (49 sets, 50 strings). Comparison covered key identity, type, value and absolute expiry. Redis 7 rollback copies matched the same snapshots.
- Env-gated Go tests against the real Redis 8 container passed for `UserSessionCache` Set/Exists/TTL/Delete, fixed-window Lua rate limiting through expiry/reset, and AuthService refresh-family Lua rotation/replay/revocation. Normal Go tests skip these when `BODYSENSE_REDIS8_ADDR` is unset. JobRuntime/run leases use PostgreSQL repository state.
- The five image declarations agree on `redis:8.10.2-alpine` and upstream digest `sha256:3811787313eba226a2ef38658c6ccb91cd5e110edc89c37767de373120a0e5a0`; the manual ACR mirror target matches the production Compose tag. The mirror was not run and no production image or runtime was changed.
- Gates passed: focused Go auth/cache/service tests; `pnpm contracts:verify`; `bash scripts/validate-supply-chain.sh`; `pnpm test:delivery` (43 tests); dev, staging and production Compose config validation; Redis image/mirror contract check; `git diff --check`.
- Disposable b081 runtime and temporary artifacts were removed after evidence capture. No live staging or production deployment was performed.

---

## BS-UPG-082 — Upgrade LiteLLM and Web runtime proxy image

**Goal:** refresh infrastructure services without altering application contracts.

**Scope:**

- LiteLLM 1.97.0 → 1.102.1 candidate;
- nginx 1.27 → 1.30.5 stable candidate;
- refresh Caddy 2 mirror/digest.

**Implementation:**

1. Upgrade LiteLLM in dev/staging.
2. Verify configured providers, retries, fallback, health, streaming.
3. Upgrade nginx stable runtime image.
4. Verify:
   - SPA fallback;
   - API proxy;
   - cache headers;
   - CSP/TAO/CORS behavior for static assets;
   - gzip/brotli behavior where configured.
5. Refresh Caddy digest without changing routing semantics.
6. Re-run staging smoke.

**Acceptance:** model routing and Web/API transport remain behaviorally equivalent.

---

## BS-UPG-090 — Refresh CI actions and immutable supply-chain pins

**Goal:** make CI/tooling reflect the upgraded runtime and remove version drift.

Known candidates from audit include:

```text
astral-sh/setup-uv       10.1.0 -> 10.2.0
docker/build-push-action 7.3.0  -> 7.4.0
docker/setup-buildx      4.3.0  -> 4.4.1
```

**Implementation:**

1. Update action SHAs with version comments.
2. Keep actions pinned by immutable SHA.
3. Align setup-node/setup-go/toolchain inputs with the new baselines.
4. Pin refreshed production base images by digest where repository policy requires it.
5. Run workflow syntax/governance checks.

**Acceptance:** no floating action references are introduced.

---

## BS-UPG-100 — Repository-wide convergence gate

**Goal:** prove all upgrade lanes work together.

Required checks:

```bash
pnpm install --frozen-lockfile

pnpm lint
pnpm typecheck
pnpm test
pnpm build

pnpm contracts:verify
pnpm postman:verify
pnpm quality:verify

pnpm test:static-assets
pnpm test:delivery
pnpm test:validation-lifecycle

git diff --check
```

API:

```bash
cd apps/api
go test ./...
go vet ./...
```

AI:

```bash
cd apps/ai-service
uv sync --frozen --all-extras
uv run ruff check src tests
uv run pyright
uv run pytest
```

Deployment:

```bash
pnpm validate:local-deploy
```

Remote-dev performance:

- classic fallback still starts;
- bundled host-dev starts;
- request graph is materially collapsed;
- TSX HMR works;
- Tailwind HMR works;
- no unresolved workspace module.

**Acceptance:** complete green evidence recorded in this plan before staging promotion.

---

## BS-UPG-110 — Canonical staging qualification

**Goal:** validate the exact integrated candidate in production-shaped staging.

**Checks:**

1. deploy exact SHA to canonical GCP staging;
2. DB/Redis/LiteLLM/API/AI/Web health;
3. authentication session lifecycle;
4. onboarding/profile;
5. consultation streaming;
6. Diagnosis/Treatment smoke;
7. health-document OCR path;
8. 3D Explorer load + region interaction;
9. static CDN/asset timing;
10. remote/browser console error audit;
11. staging rollback drill for infrastructure-image failures.

**Acceptance:** zero blocker regression and explicit staging evidence.

---

## BS-UPG-120 — Production release and post-release verification

**Goal:** deploy only the already-qualified exact candidate.

**Preconditions:**

- BS-UPG-000 through BS-UPG-110 closed;
- full CI green;
- candidate images immutable;
- DB and Redis backups verified;
- production rollback path ready.

**Implementation:**

1. publish release candidate/exact SHA via current delivery lifecycle;
2. explicitly select for production;
3. deploy production;
4. verify health/version/schema;
5. smoke user paths;
6. inspect logs/restarts/capacity;
7. verify CDN/static assets;
8. verify Redis persistence and PostgreSQL health;
9. record final release evidence.

---

# 7. Dependency/order map

```text
UPG-000
  ↓
UPG-010
  ↓
UPG-020 → UPG-021
  ↓
UPG-030
  ├─→ UPG-040
  └─→ UPG-041
        ↓
      UPG-050
        ↓
UPG-060   UPG-070 → UPG-071 → UPG-072
    \        /
     \      /
      UPG-080 → UPG-081 → UPG-082
              ↓
           UPG-090
              ↓
           UPG-100
              ↓
           UPG-110
              ↓
           UPG-120
```

Go and Python dependency refreshes can be developed independently after the shared toolchain baseline, but they must converge before infrastructure and full release validation.

---

# 8. Verification matrix

| Boundary | Focused verification | Wide verification |
| --- | --- | --- |
| Vite Bundled Dev | config tests, request-count probe, HMR | Web test/build + browser smoke |
| React | route/auth/query tests | Web full suite |
| assistant-ui / AI SDK | Consultation streaming/projection | Web + E2E |
| Three/R3F | BodyExplorer tests/visual smoke | build chunk budget + staging |
| Vitest 5 | representative suite + config | all JS tests |
| GraphQL 17 | GraphQL lab | root test/build |
| Go deps | affected packages | `go test ./...` + vet |
| Python base deps | RAG/OCR/API focused | full pytest + Ruff + Pyright |
| PydanticAI | Agent focused tests/evals | Python full + local deploy |
| OpenAI 3 | embedding/ASR/gateway | Python full + staging model smoke |
| PostgreSQL patch | migration/replay/restore | local deploy + staging |
| Redis 8 | AOF/session/rate-limit | local deploy + staging rollback |
| LiteLLM | routing/stream/fallback | Consultation/Diagnosis staging |
| nginx/Caddy | proxy/static headers | staging browser smoke |
| CI actions | workflow audit | GitHub CI |

---

# 9. Rollback model

Rollback is lane-specific.

## Web/package lanes

Revert package manifest + lockfile + compatibility edits for the failing lane only.

## Bundled Dev

Set:

```env
BODYSENSE_VITE_BUNDLED_DEV=false
```

and return immediately to classic Vite.

## Go/Python

Revert the dependency group and lock/module files. No DB schema migration is required merely for library upgrades.

## PostgreSQL

Remain on PostgreSQL 18; patch rollback uses the pre-upgrade database backup/image only if required by validation.

## Redis

Do not attempt blind in-place major downgrade using a Redis-8-mutated volume. Rollback uses the pre-upgrade volume/AOF snapshot captured before first Redis 8 startup.

## Production images

Current release lifecycle and exact-SHA/digest rollback remain authoritative.

---

# 10. Explicit holds / deferred upgrades

The following are **not** part of the implementation target even if newer versions exist:

| Technology | Hold |
| --- | --- |
| Node 26 | Current rather than LTS; no need for this program |
| pnpm 12 | package-manager major; no current product value |
| Python 3.14 | native OCR/ML/MediaPipe/ONNX compatibility should mature separately |
| PostgreSQL 19 beta | beta, not production target |
| TypeScript 7 as sole canonical ecosystem | typescript-eslint currently does not declare TS7 support |
| Vanatome anatomy release | anatomy data promotion needs its own semantic/visual qualification |
| broad import/barrel rewrite | does not address the root remote-dev mechanism |
| architecture/domain refactor | unrelated to stack modernization |

---

# 11. Completion definition

This active plan is complete only when all of the following are true:

1. GCP Dev toolchain matches repository runtime lines.
2. BodySense Web remote host-dev uses Vite Bundled Dev by default with classic fallback.
3. Remote browser module waterfall is materially reduced and measured.
4. Target JS/React/AI/3D/test tooling upgrades are applied and verified.
5. Go direct dependencies are refreshed on Go 1.26.8.
6. Python 3.13.15 and selected AI dependencies are upgraded with Agent/eval parity.
7. PostgreSQL 18 patch level is current and validated.
8. Redis 8 upgrade is proven with persistence + rollback evidence.
9. LiteLLM/nginx/runtime images are refreshed.
10. CI actions and image pins are converged.
11. Full repository + local-deploy gate passes.
12. Canonical staging passes production-shaped qualification.
13. Production is only updated from that accepted exact candidate.
14. The plan is updated with final evidence and moved to `docs/plan/archive/`.

---

# 12. First implementation checkpoint

When implementation starts, the first commit/batch should include only:

```text
BS-UPG-000 baseline evidence
BS-UPG-010 toolchain convergence
BS-UPG-020 Vite/Nx/plugin foundation
BS-UPG-021 Bundled Dev remote lane
```

Do **not** begin React/AI/3D/runtime-image mass upgrades until the remote-development foundation is independently green.

That creates a clean early checkpoint:

```text
modern host toolchain
+
Vite 8.3
+
MemoFlow-derived Bundled Dev
+
classic fallback
+
measured remote improvement
```

After that checkpoint, proceed through the remaining upgrade batches without changing the program scope.
