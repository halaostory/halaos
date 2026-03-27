import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Approvals', () => {
  test('page loads', async ({ adminPage: page }) => {
    await page.goto(BASE + '/approvals');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('h2')).toContainText('Approvals');
  });

  test('approval list or empty state', async ({ adminPage: page }) => {
    await page.goto(BASE + '/approvals');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Either the data table with pending approvals or the NaiveUI empty state should be visible
    const table = page.locator('.n-data-table');
    const emptyState = page.locator('.n-empty');
    await expect(table.or(emptyState).first()).toBeVisible({ timeout: 15_000 });
  });
});
