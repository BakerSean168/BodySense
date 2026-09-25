FROM node:24-slim
RUN corepack enable && corepack prepare pnpm@11.27.1 --activate
WORKDIR /app

# Supply-chain validation depends on authoritative registry metadata.
# Default to the canonical npm registry; China-local builds may explicitly
# override with --build-arg NPM_REGISTRY=https://registry.npmmirror.com.
ARG NPM_REGISTRY=https://registry.npmjs.org
RUN pnpm config set registry ${NPM_REGISTRY}

# Copy workspace root files for pnpm workspace resolution
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY apps/web/package.json apps/web/
COPY packages/contracts/package.json packages/contracts/
COPY packages/utils/package.json packages/utils/

RUN pnpm install --no-frozen-lockfile

# Linux native bindings are resolved from the committed lockfile.
# Do not hide missing optional bindings behind a best-effort post-install mutation.

# Copy source
COPY . .

EXPOSE 5173
CMD ["sh", "-c", "cd apps/web && npx vite --host 0.0.0.0"]
