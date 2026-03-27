import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('DTR (Daily Time Record)', () => {
  test('page loads', async ({ adminPage: page }) => {
    await page.goto(BASE + '/dtr');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // DTR page should show heading or content
    const heading = page.locator('h2');
    await expect(heading).toBeVisible({ timeout: 15_000 });
  });

  test('content visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/dtr');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Date range picker or data table should be visible
    const datePicker = page.locator('.n-date-picker');
    const table = page.locator('.n-data-table');
    await expect(datePicker.or(table).first()).toBeVisible({ timeout: 15_000 });
  });
});
