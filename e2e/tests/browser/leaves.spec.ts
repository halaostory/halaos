import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Leave Management', () => {
  test('page loads', async ({ adminPage: page }) => {
    await page.goto(BASE + '/leaves');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('h2')).toContainText('Leave Management');
  });

  test('leave list visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/leaves');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Either the data table with leave records or tabs should be visible
    const table = page.locator('.n-data-table');
    const tabs = page.locator('.n-tabs');
    await expect(table.or(tabs).first()).toBeVisible({ timeout: 15_000 });
  });

  test('apply leave button visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/leaves');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    const applyButton = page.locator('button').filter({ hasText: 'Apply Leave' });
    await expect(applyButton).toBeVisible({ timeout: 15_000 });
  });
});
