import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Salary Configuration', () => {
  test('page loads without error', async ({ adminPage: page }) => {
    await page.goto(BASE + '/salary');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('heading and tabs visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/salary');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('h2')).toBeVisible();
    // Tabs: Structures, Components
    await expect(page.locator('.n-tabs')).toBeVisible();
  });

  test('data table renders', async ({ adminPage: page }) => {
    await page.goto(BASE + '/salary');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('.n-data-table')).toBeVisible();
  });
});
