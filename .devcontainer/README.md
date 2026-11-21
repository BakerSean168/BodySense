# BodySense Codespaces / temporary Agent workers

Two profiles are available for the polyglot Nx repository:

| Profile | Path | Installed |
| --- | --- | --- |
| **Lite** | `.devcontainer/devcontainer.json` | Node 24.22.1, pnpm 11.27.1, GH CLI, SSHD |
| **Full** | `.devcontainer/full/devcontainer.json` | Lite + Go 1.26, Python 3.13, uv, Docker-in-Docker / Compose |

Lite is for frontend edits and focused tests. Full is for the Go API, Python AI, and database-backed tests. The Full profile uses a prebuilt Python 3.13 image to avoid compiling Python when creating a worker. It includes uv/uvx from the same immutable image digest as `apps/ai-service/Dockerfile`. Toolchain versions align with `package.json`, `apps/api/go.mod` and `apps/ai-service/pyproject.toml`. Both profiles use unprivileged `vscode`, include SSHD for `gh codespace ssh`, and do not impose hidden CPU/memory limits.

## Create a worker

```bash
# One-time account-owner authorization on the trusted controller:
gh auth refresh -h github.com -s codespace

# Lite, normally 2-core:
gh codespace create -R BakerSean168/BodySense -b main \
  --devcontainer-path .devcontainer/devcontainer.json \
  --idle-timeout 10m --retention-period 24h

# Full: select a suitable machine (often 4+ cores):
gh codespace create -R BakerSean168/BodySense -b main \
  --devcontainer-path .devcontainer/full/devcontainer.json \
  --idle-timeout 10m --retention-period 24h
```

Connect with `gh codespace ssh -c NAME`. Local Dev Containers builds verify the image and toolchains; a successful GitHub Codespace creation and SSH command are separate acceptance steps requiring the controller account’s `codespace` authorization. Work on a task branch, commit/push, and then stop/delete with `gh codespace stop -c NAME` / `gh codespace delete -c NAME`.

## On-demand setup

```bash
bash .devcontainer/post-create.sh --check
pnpm install --frozen-lockfile
pnpm exec nx affected -t lint,typecheck,test --base=origin/main
```

Full's `--full-check` verifies Go, Python, uv and Docker Compose. Install Python dependencies on demand with `cd apps/ai-service && uv sync --frozen`; for Go, `cd apps/api && go test ./...`. Start local Compose only if required (`pnpm dev:infra:up`), then stop it. Never run production workloads in Codespaces.

No automatic pnpm installation, Go/Python dependency downloads, ML models, Compose startup, swap creation, env copying, or production secrets. Use task-scoped Codespaces secrets, not production credentials. Keep local database ports private.

GitHub Education verified students receive **180 core-hours and 20 GB-month of storage per month** for personal-account Codespaces, shared across all repos. Verify remaining quota and billing safeguards before creating workers. No prebuilds are enabled.

Default forwarded development endpoints follow `scripts/dev-env.sh`: Web `20100`; Full also includes API `20101`, AI `20102` and Document `20103`. PostgreSQL `20110` and Redis `20111` remain private.

## Container acceptance

The `Dev Containers` workflow builds and starts both original profiles on an isolated GitHub runner, runs their automatic postCreate commands, and checks the development user, exact Node/pnpm versions, GitHub CLI and the SSH listener. Full also verifies the running Docker daemon, Compose and a small build that needs no image download. It additionally checks Go, Python 3.13 and uv/uvx.

Run `bash .devcontainer/verify.sh lite` or `bash .devcontainer/verify.sh full` inside the corresponding container for the same checks. The workflow uses no repository secrets, installs no monorepo dependencies, creates no Codespace, and removes its container and dedicated volumes. It also runs when root `package.json` changes so pnpm drift is detected.

Both matrix results must pass alongside the repository's required CI check before accepting a container change. Successful runner validation does not authorize or prove a real Codespace session; `gh codespace create` and an Agent/Mius SSH connection still require the controller account's Codespaces authorization.
