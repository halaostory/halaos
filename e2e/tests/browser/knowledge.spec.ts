import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Knowledge Base', () => {
  test('page loads without error', async ({ adminPage: page }) => {
    await page.goto(BASE + '/knowledge');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('card with title and search visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/knowledge');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Knowledge base is wrapped in NCard
    await expect(page.locator('.n-card')).toBeVisible();
    // Category filter select
    await expect(page.locator('.n-select')).toBeVisible();
    // Search input
    await expect(page.locator('.n-input')).toBeVisible();
  });

  test('data table renders', async ({ adminPage: page }) => {
    await page.goto(BASE + '/knowledge');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('.n-data-table')).toBeVisible();
  });
});
