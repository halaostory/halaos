import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Overtime', () => {
  test('page loads', async ({ adminPage: page }) => {
    await page.goto(BASE + '/overtime');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('h2')).toContainText('Overtime');
  });

  test('apply overtime button visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/overtime');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    const applyButton = page.locator('button', { hasText: 'Apply Overtime' });
    await expect(applyButton).toBeVisible();
  });
});
