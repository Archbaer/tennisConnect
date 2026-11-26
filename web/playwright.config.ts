import { defineConfig, devices } from '@playwright/test';

const databaseURL =
  process.env.TEST_DATABASE_URL ??
  'postgres://tennisconnect:local-development-only@localhost:5432/tennisconnect_test?sslmode=disable';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  globalSetup: './e2e/global-setup.ts',
  reporter: [['list']],
  use: {
    baseURL: 'http://127.0.0.1:4173',
    trace: 'retain-on-failure'
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
      grepInvert: /@mobile/
    },
    {
      name: 'mobile-chromium',
      use: { ...devices['Pixel 5'] },
      grep: /@mobile/
    }
  ],
  webServer: [
    {
      command: 'npm run dev -- --host 127.0.0.1 --port 4173',
      url: 'http://127.0.0.1:4173',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000
    },
    {
      command: 'go run ./cmd/api',
      cwd: '..',
      url: 'http://127.0.0.1:8080/healthz',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      env: {
        DATABASE_URL: databaseURL,
        APP_BASE_URL: 'http://127.0.0.1:4173',
        EMAIL_FROM: 'Tennis Connect <auth@example.com>',
        RESEND_API_KEY: 'test-only',
        EMAIL_ENCRYPTION_KEY: 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=',
        EMAIL_HMAC_KEY: 'AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=',
        COOKIE_SECURE: 'false',
        TRUST_PROXY_HEADERS: 'false',
        AWS_EC2_METADATA_DISABLED: 'true',
        S3_BUCKET: 'e2e-bucket',
        AWS_REGION: 'eu-west-2',
        AWS_ACCESS_KEY_ID: 'test-only',
        AWS_SECRET_ACCESS_KEY: 'test-only',
        PORT: '8080'
      }
    }
  ]
});
