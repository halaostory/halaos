import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Clearance', () => {
  test('page loads and content is visible', async ({ adminPage: page }) => {
    const response = await page.goto(BASE + '/clearance');
    expect(response?.status()).not.toBe(404);
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('body')).toBeVisible();
  });
});
