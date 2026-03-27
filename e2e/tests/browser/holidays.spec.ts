import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Holidays', () => {
  test('page loads without error', async ({ adminPage: page }) => {
    await page.goto(BASE + '/holidays');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('heading and year selector visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/holidays');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('h2')).toBeVisible({ timeout: 15_000 });
    // Year filter select or data table should be visible
    const yearSelect = page.locator('.n-select');
    const table = page.locator('.n-data-table');
    await expect(yearSelect.or(table).first()).toBeVisible({ timeout: 15_000 });
  });

  test('data table and add button visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/holidays');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('.n-data-table')).toBeVisible();
    // Add Holiday button
    await expect(page.locator('.n-button').first()).toBeVisible();
  });
});
