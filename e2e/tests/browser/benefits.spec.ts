import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Benefits', () => {
  test('page loads without error', async ({ adminPage: page }) => {
    await page.goto(BASE + '/benefits');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('body')).toBeVisible();
    // Should not show 404
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('main heading and tabs visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/benefits');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Page heading
    await expect(page.locator('h2')).toBeVisible();
    // Benefits page uses NTabs for My Benefits, Plans, Enrollments, Claims
    await expect(page.locator('.n-tabs')).toBeVisible();
  });

  test('data table renders', async ({ adminPage: page }) => {
    await page.goto(BASE + '/benefits');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('.n-data-table')).toBeVisible();
  });
});
