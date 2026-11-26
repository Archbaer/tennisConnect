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
      }),
      csp: {
        mode: 'hash',
        directives: {
          'default-src': ['self'],
          'script-src': ['self'],
          'style-src': ['self', 'unsafe-inline'],
          'img-src': ['self', 'data:', 'https://*.s3.eu-west-2.amazonaws.com'],
          'connect-src': ['self', 'https://*.s3.eu-west-2.amazonaws.com'],
          'font-src': ['self'],
          'object-src': ['none'],
          'base-uri': ['none'],
          'form-action': ['self']
        }
      }
    })
  ],
  resolve: {
    alias: { '#lib': lib }
  },
  server: {
    proxy: {
      '/api': api,
      '/auth': api,
      '/reviewer/verify': api,
      '/healthz': api
    }
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts']
  }
});
