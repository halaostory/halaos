import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Payroll', () => {
  test('page loads', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/payroll');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toContainText('Payroll');
  });

  test('content visible (cycles list or dashboard)', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/payroll');
    await page.waitForLoadState('networkidle');

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
