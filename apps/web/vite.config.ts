/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

// Strip 'use client' directives from @base-ui/react modules
function stripUseClient(): import("vite").Plugin {
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

const webRoot = path.resolve(__dirname);
const configuredAssetBase = process.env.VITE_ASSET_BASE?.trim();
const assetBase = configuredAssetBase
  ? `${configuredAssetBase.replace(/\/+$/, "")}/`
  : "/";
const allowedHosts = (process.env.BODYSENSE_ALLOWED_HOSTS ?? "")
  .split(",")
  .map((host) => host.trim())
  .filter(Boolean);

export default defineConfig({
  root: webRoot,
  // Production may serve immutable hashed assets from a public CDN while the
  // HTML/API/SSE origin remains private. Vite rewrites entry assets and dynamic
  // imports to this base; local development keeps the normal same-origin '/'.
  base: assetBase,
  plugins: [stripUseClient(), react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(webRoot, "src"),
    },
    dedupe: ["react", "react-dom"],
  },
  optimizeDeps: {
    include: ["@base-ui/react", "@floating-ui/react-dom", "@floating-ui/utils"],
  },
  ssr: {
    noExternal: ["@base-ui/react"],
  },
  server: {
    ...(allowedHosts.length > 0 ? { allowedHosts } : {}),
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
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: ["src/test-setup.ts"],
    globals: true,
    // Vitest 4: unit suite stays on happy-dom; browser mode is opt-in later.
    restoreMocks: true,
  },
});
