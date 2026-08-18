/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // Same-origin in development, exactly as nginx serves it in the
      // container. Nothing cross-origin happens, so CORS never applies.
      '/v1': {
        target: process.env.VITE_API_TARGET ?? 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    coverage: {
      provider: 'v8',
      // `lcovonly` rather than `lcov`: the latter also emits its own copy of the
      // HTML report, and the reports are committed, so that would double what
      // is tracked for no gain.
      reporter: ['text-summary', 'html', 'lcovonly'],
      include: ['src/**/*.{ts,tsx}'],
      exclude: [
        // Entry point and barrels: wiring with no behaviour of their own.
        'src/main.tsx',
        'src/**/index.ts',
        // Types erase at compile time; there is nothing to execute.
        'src/types/**',
        'src/api/types.ts',
        'src/vite-env.d.ts',
        // Reads import.meta.env and constructs the singleton; covered by the
        // client's own tests, which inject their configuration.
        'src/api/config.ts',
      ],
      // Matches the backend's gate so neither side can quietly fall behind.
      thresholds: {
        statements: 85,
        branches: 85,
        functions: 85,
        lines: 85,
      },
    },
  },
})
