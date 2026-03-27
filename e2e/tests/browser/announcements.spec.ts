import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Announcements', () => {
  test('page loads without error', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/announcements');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('heading visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/announcements');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toBeVisible();
  });

  test('create button visible for admin', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/announcements');
    await page.waitForLoadState('networkidle');
    // Admin should see Create button
    await expect(page.locator('.n-button').first()).toBeVisible();
  });
});
