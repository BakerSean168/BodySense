/// <reference types="vitest/config" />
import {
  defineConfig,
  type ConfigEnv,
  type HotUpdateOptions,
  type Plugin,
  type UserConfig,
} from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";
import { realpathSync } from "node:fs";
import { gzipSync } from "node:zlib";

// Strip 'use client' directives from @base-ui/react modules
function stripUseClient(): Plugin {
  return {
    name: "strip-use-client",
    enforce: "pre",
    transform(code, id) {
      if (id.includes("@base-ui") && code.startsWith("'use client'")) {
        return code.slice("'use client';".length);
      }
      return code;
    },
  };
}

function enforceChunkBudgets(): Plugin {
  const defaultRawLimit = 500_000;
  const bodyExplorerRawLimit = 1_300_000;
  const bodyExplorerGzipLimit = 300_000;
  // Three contains a single >500 kB source module, so once two independent lazy
  // 3D views share it Rolldown must emit a common lazy-only runtime chunk. Keep
  // the global 500 kB budget intact and govern that unavoidable runtime explicitly.
  const body3DThreeRawLimit = 900_000;
  const body3DThreeGzipLimit = 300_000;

  return {
    name: "bodysense-web-chunk-budgets",
    apply: "build",
    generateBundle(_options, bundle) {
      for (const output of Object.values(bundle)) {
        if (output.type !== "chunk") continue;

        const rawBytes = Buffer.byteLength(output.code, "utf8");
        const isBodyExplorer3D = output.name === "BodyExplorer3D";
        const isBody3DThreeRuntime = output.name === "Body3DThreeRuntime";

        if (isBody3DThreeRuntime) {
          const onlyThreeModules = Object.keys(output.modules).every((id) =>
            /node_modules[\\/]three[\\/]/.test(id),
          );
          const staticallyReachableFromEntry = (fileName: string, seen = new Set<string>()): boolean => {
            if (seen.has(fileName)) return false;
            seen.add(fileName);
            return Object.values(bundle).some((candidate) => {
              if (candidate.type !== "chunk" || !candidate.imports.includes(fileName)) return false;
              return candidate.isEntry || staticallyReachableFromEntry(candidate.fileName, seen);
            });
          };
          const gzipBytes = gzipSync(output.code).byteLength;
          if (!onlyThreeModules || staticallyReachableFromEntry(output.fileName) || rawBytes > body3DThreeRawLimit || gzipBytes > body3DThreeGzipLimit) {
            this.error(
              `Body3DThreeRuntime must remain Three-only, lazy-only, and within its explicit budget: ` +
                `${rawBytes} raw / ${gzipBytes} gzip bytes.`,
            );
          }
          continue;
        }

        if (!isBodyExplorer3D && rawBytes > defaultRawLimit) {
          const largestModules = Object.entries(output.modules)
            .map(([id, meta]) => ({ id, bytes: meta.renderedLength }))
            .sort((left, right) => right.bytes - left.bytes)
            .slice(0, 6)
            .map(({ id, bytes }) => `${bytes}:${id}`)
            .join(", ");
          this.error(
            `Unexpected Web chunk ${output.fileName} is ${rawBytes} bytes; ` +
              `the default production budget is ${defaultRawLimit} bytes. ` +
              `Largest modules: ${largestModules}`,
          );
        }

        if (!isBodyExplorer3D) continue;
        if (!output.isDynamicEntry) {
          this.error("BodyExplorer3D must remain a lazy dynamic entry.");
        }

        const gzipBytes = gzipSync(output.code).byteLength;
        if (
          rawBytes > bodyExplorerRawLimit ||
          gzipBytes > bodyExplorerGzipLimit
        ) {
          this.error(
            `BodyExplorer3D exceeds its explicit lazy-viewer budget: ` +
              `${rawBytes} raw / ${gzipBytes} gzip bytes; limits are ` +
              `${bodyExplorerRawLimit} raw / ${bodyExplorerGzipLimit} gzip bytes.`,
          );
        }
      }
    },
  };
}

type Environment = Record<string, string | undefined>;

export function shouldUseBundledDev(
  { command, mode }: Pick<ConfigEnv, "command" | "mode">,
  env: Environment = process.env,
): boolean {
  return (
    command === "serve" &&
    mode === "development" &&
    env.BODYSENSE_VITE_BUNDLED_DEV === "true" &&
    env.NODE_ENV !== "test"
  );
}

/**
 * Tailwind CSS 4.3.x currently expects the classic Vite dev-server `server`
 * object in its hotUpdate hook. Vite Bundled Dev intentionally invokes the hook
 * with a smaller context. In bundled mode we skip only that classic-server
 * invalidation helper when `server` is absent; Tailwind transforms still run
 * during bundled regeneration.
 */
export function adaptTailwindPluginsForBundledDev(
  plugins: Plugin[],
  useBundledDev: boolean,
): Plugin[] {
  if (!useBundledDev) return plugins;

  return plugins.map((plugin) => {
    if (
      plugin.name !== "@tailwindcss/vite:generate:serve" ||
      typeof plugin.hotUpdate !== "function"
    ) {
      return plugin;
    }

    const upstreamHotUpdate = plugin.hotUpdate;
    return {
      ...plugin,
      hotUpdate(options) {
        if (!(options as Partial<HotUpdateOptions>).server) return;
        return upstreamHotUpdate.call(this, options);
      },
    };
  });
}

function createTailwindPlugins(useBundledDev: boolean): Plugin[] {
  return adaptTailwindPluginsForBundledDev(tailwindcss(), useBundledDev);
}

const webRoot = path.resolve(import.meta.dirname);
const repositoryRoot = path.resolve(webRoot, "../..");
const dependencyRoot = realpathSync(path.join(repositoryRoot, "node_modules"));

export function createBodySenseViteConfig({
  mode,
  command,
}: ConfigEnv): UserConfig {
  const configuredAssetBase = process.env.VITE_ASSET_BASE?.trim();
  const assetBase = configuredAssetBase
    ? `${configuredAssetBase.replace(/\/+$/, "")}/`
    : "/";
  const allowedHosts = (process.env.BODYSENSE_ALLOWED_HOSTS ?? "")
    .split(",")
    .map((host) => host.trim())
    .filter(Boolean);
  const useBundledDev = shouldUseBundledDev({ command, mode });

  return {
    root: webRoot,
    // Production may serve immutable hashed assets from a public CDN while the
    // HTML/API/SSE origin remains private. Vite rewrites entry assets and dynamic
    // imports to this base; local development keeps the normal same-origin '/'.
    base: assetBase,
    experimental: {
      bundledDev: useBundledDev,
    },
    plugins: [
      stripUseClient(),
      react(),
      ...createTailwindPlugins(useBundledDev),
      enforceChunkBudgets(),
    ],
    resolve: {
      alias: {
        "@": path.resolve(webRoot, "src"),
        "@bodysense/contracts": path.resolve(
          webRoot,
          "../../packages/contracts/src/index.ts",
        ),
      },
      dedupe: ["react", "react-dom"],
    },
    optimizeDeps: {
      include: [
        "@base-ui/react",
        "@floating-ui/react-dom",
        "@floating-ui/utils",
      ],
    },
    ssr: {
      noExternal: ["@base-ui/react"],
    },
    build: {
      rolldownOptions: {
        output: {
          codeSplitting: {
            groups: [
              {
                name: "Body3DThreeRuntime",
                test: /node_modules[\\/]three[\\/]/,
                priority: 30,
              },
            ],
          },
        },
      },
      // BodyExplorer3D is intentionally isolated behind React.lazy. Vite's generic
      // warning cannot express that exception, so the custom plugin above keeps a
      // 500 kB default budget for every other chunk and a tighter raw+gzip budget
      // for the lazy 3D viewer itself.
      chunkSizeWarningLimit: 1300,
    },
    server: {
      ...(allowedHosts.length > 0 ? { allowedHosts } : {}),
      // Worktrees may share the repository dependency tree through a symlink.
      // Vite resolves font URLs to the real path, so permit exactly the repo and
      // the resolved node_modules root instead of widening @fs to /home/dev/projects.
      fs: {
        allow: [repositoryRoot, dependencyRoot],
      },
      // Direct dev stays loopback-only. Tailscale Serve owns the Tailnet address
      // on the same project port and proxies into this listener.
      host: process.env.BODYSENSE_WEB_HOST || "127.0.0.1",
      port: Number(process.env.BODYSENSE_WEB_PORT || 5173),
      strictPort: true,
      proxy: {
        "/api": {
          target: process.env.VITE_DEV_API_TARGET || "http://localhost:8080",
          changeOrigin: true,
        },
      },
    },
    test: {
      root: webRoot,
      environment: "happy-dom",
      include: ["src/**/*.test.{ts,tsx}", "vite.config.test.ts"],
      setupFiles: ["src/test-setup.ts"],
      globals: true,
      restoreMocks: true,
    },
  };
}

export default defineConfig(createBodySenseViteConfig);
