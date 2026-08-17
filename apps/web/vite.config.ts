/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import path from 'node:path';

// Strip 'use client' directives from @base-ui/react modules
function stripUseClient(): import('vite').Plugin {
  return {
    name: 'strip-use-client',
    enforce: 'pre',
    transform(code, id) {
      if (id.includes('@base-ui') && code.startsWith("'use client'")) {
        return code.slice("'use client';".length);
      }
      return code;
    },
  };
}

const webRoot = path.resolve(__dirname);

export default defineConfig({
  root: webRoot,
  plugins: [stripUseClient(), react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(webRoot, 'src'),
    },
    dedupe: ['react', 'react-dom'],
  },
  optimizeDeps: {
    include: ['@base-ui/react', '@floating-ui/react-dom', '@floating-ui/utils'],
  },
  ssr: {
    noExternal: ['@base-ui/react'],
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_DEV_API_TARGET || 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    root: webRoot,
    environment: 'happy-dom',
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    globals: true,
    // Vitest 4: unit suite stays on happy-dom; browser mode is opt-in later.
    restoreMocks: true,
  },
});
