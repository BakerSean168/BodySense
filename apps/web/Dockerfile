FROM node:24-slim
RUN corepack enable && corepack prepare pnpm@11 --activate
WORKDIR /app

# Use npm mirror for faster downloads
RUN npm config set registry https://registry.npmmirror.com

# Copy workspace root files for pnpm workspace resolution
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY apps/web/package.json apps/web/
COPY packages/contracts/package.json packages/contracts/
COPY packages/utils/package.json packages/utils/

RUN pnpm install --no-frozen-lockfile

# Install Linux native bindings that may be missing from Windows-generated lock file
RUN pnpm add -w @rolldown/binding-linux-x64-gnu @esbuild/linux-x64 lightningcss-linux-x64-gnu --no-save || true

# Copy source
COPY . .

EXPOSE 5173
CMD ["sh", "-c", "cd apps/web && npx vite --host 0.0.0.0"]
