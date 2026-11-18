import adapter from '@sveltejs/adapter-static';
import { fileURLToPath } from 'node:url';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

const api = process.env.API_ORIGIN ?? 'http://localhost:8080';
const lib = fileURLToPath(new URL('./src/lib', import.meta.url));

export default defineConfig({
  plugins: [
    sveltekit({
      adapter: adapter({
        fallback: 'index.html',
        precompress: false,
        strict: true
      })
    })
  ],
  resolve: {
    alias: { '#lib': lib }
  },
  server: {
    proxy: {
      '/api': api,
      '/auth': api,
      '/reviewer': api,
      '/healthz': api
    }
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts']
  }
});
