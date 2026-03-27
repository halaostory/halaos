import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Payroll', () => {
  test('page loads', async ({ adminPage: page }) => {
    await page.goto(BASE + '/payroll');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('h2')).toContainText('Payroll');
  });

  test('content visible (cycles list or dashboard)', async ({ adminPage: page }) => {
    await page.goto(BASE + '/payroll');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Tabs should be visible with the Payroll cycles tab
    const tabs = page.locator('.n-tabs');
    await expect(tabs).toBeVisible();

    // Either the data table with cycles or the create cycle button should be present
    const createButton = page.locator('button', { hasText: 'Create Cycle' });
    const table = page.locator('.n-data-table');
    const hasButton = await createButton.isVisible().catch(() => false);
    const hasTable = await table.isVisible().catch(() => false);

    expect(hasButton || hasTable).toBe(true);
  });
});
