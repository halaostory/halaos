import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Announcements', () => {
  test('page loads without error', async ({ adminPage: page }) => {
    await page.goto(BASE + '/announcements');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('heading visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/announcements');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('h2')).toBeVisible();
  });

  test('create button visible for admin', async ({ adminPage: page }) => {
    await page.goto(BASE + '/announcements');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Admin should see Create button
    await expect(page.locator('.n-button').first()).toBeVisible();
  });
});
