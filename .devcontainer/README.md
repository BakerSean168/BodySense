# BodySense: ephemeral GitHub Codespaces workers

BodySense is a polyglot Nx repository. It requires **Node >=24**, **pnpm 11.27.1**, **Go 1.26** (API), and **Python 3.13** (AI service). The two development profiles intentionally differ:

| Profile | Configuration | What is preinstalled |
| --- | --- | --- |
| **Lite (default)** | `.devcontainer/devcontainer.json` | Node 24.22.1, pnpm 11.27.1, GitHub CLI, SSHD |
| **Full** | `.devcontainer/full/devcontainer.json` | Above + Go 1.26, Python 3.13, Docker-in-Docker and Compose |

Lite is designed for quick edits, frontend checks and focused tests; it does not start or install backend services. Full is for API/AI work, database-backed tests, and local Docker Compose. The Full image uses a prebuilt Python 3.13 image to avoid compiling Python during Codespace creation. Both profiles use an unprivileged `vscode` user and intentionally do **not** hardcode CPU/RAM limits: choose the machine size in GitHub.

## Create and connect

```bash
# One-time account-owner authorization on the trusted controller
gh auth refresh -h github.com -s codespace

# Lite (default, typically 2 cores)
gh codespace create -R BakerSean168/BodySense -b main \
  --devcontainer-path .devcontainer/devcontainer.json \
  --idle-timeout 10m --retention-period 24h

# Full (choose sufficient resources, typically 4 cores or more)
gh codespace create -R BakerSean168/BodySense -b main \
  --devcontainer-path .devcontainer/full/devcontainer.json \
  --idle-timeout 10m --retention-period 24h
```

Then use `gh codespace list` / `gh codespace ssh -c NAME` / `gh codespace stop -c NAME` / `gh codespace delete -c NAME`. SSHD is included so there is no requirement for Tailscale access inside the temporary development VM.

## Install only what a task needs

```bash
bash .devcontainer/post-create.sh --check
pnpm install --frozen-lockfile
pnpm exec nx affected -t lint,typecheck,test --base=origin/main
```

For Full, `bash .devcontainer/post-create.sh --full-check` verifies Go and Python. Then optionally:

```bash
cd apps/ai-service
# Install uv for your user if it is not available, for example: pipx install uv
uv sync --frozen  # uses apps/ai-service/uv.lock
```

For the Go API, `cd apps/api && go test ./...` downloads Go dependencies on demand. For local Postgres/Redis/LiteLLM, use `pnpm dev:infra:up` **only after evaluating external service secrets and resource requirements** and shut it down when done. Browser tests / AI model downloads should never be part of the automatic Codespace creation lifecycle.

Neither profile automatically runs `pnpm install`, `uv sync`, database migrations, or `docker compose up`. Neither copies `.env.production` or binds production credentials. Provide task-scoped secrets through GitHub Codespaces secrets or a restricted external controller if required; no shared production secrets.

Verified GitHub Education students receive **180 core-hours and 20 GB-month storage per month** on personal accounts. Codespaces also have an independent maximum run lifetime; check actual quota, push the task branch, and stop/delete your worker promptly. No prebuilds are configured.
