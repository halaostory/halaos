import { defineConfig } from '@playwright/test';

const BASE_URL = process.env.E2E_BASE_URL || 'https://halaos.com';

export default defineConfig({
  globalSetup: './fixtures/data-factory.ts',
  retries: 1,
  expect: { timeout: 10_000 },
  reporter: [['html'], ['junit', { outputFile: 'test-results/junit.xml' }]],
  projects: [
    {
      name: 'api',
      testDir: './tests/api',
      use: { baseURL: BASE_URL },
      timeout: 30_000,
    },
    {
      name: 'browser',
      testDir: './tests/browser',
      use: {
        baseURL: BASE_URL,
        browserName: 'chromium',
        screenshot: 'only-on-failure',
        trace: 'on-first-retry',
      },
      timeout: 60_000,
    },
    {
      name: 'h5',
      testDir: './tests/h5',
      use: {
        baseURL: BASE_URL + '/m',
        browserName: 'chromium',
        viewport: { width: 375, height: 812 },
        screenshot: 'only-on-failure',
        trace: 'on-first-retry',
      },
      timeout: 60_000,
    },
  ],
});
