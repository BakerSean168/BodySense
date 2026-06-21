FROM node:24-alpine
RUN corepack enable && corepack prepare pnpm@11 --activate
WORKDIR /app

# Copy workspace root files for pnpm workspace resolution
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY apps/web/package.json apps/web/
COPY packages/contracts/package.json packages/contracts/
COPY packages/utils/package.json packages/utils/

RUN pnpm install --frozen-lockfile

# Copy source
COPY . .

EXPOSE 5173
CMD ["pnpm", "--filter", "@bodysense/web", "vite", "--host", "0.0.0.0"]
