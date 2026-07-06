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

export default defineConfig({
  plugins: [stripUseClient(), react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
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
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'happy-dom',
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    globals: true,
  },
});
