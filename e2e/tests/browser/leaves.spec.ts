import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Leave Management', () => {
  test('page loads', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/leaves');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toContainText('Leave Management');
  });

  test('leave list visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/leaves');
    await page.waitForLoadState('networkidle');

    // Either the data table with leave records or an empty state should be visible
    const table = page.locator('.n-data-table');
    const tabs = page.locator('.n-tabs');
    const hasTable = await table.isVisible().catch(() => false);
    const hasTabs = await tabs.isVisible().catch(() => false);

    expect(hasTable || hasTabs).toBe(true);
  });

  test('apply leave button visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/leaves');
    await page.waitForLoadState('networkidle');

    const applyButton = page.locator('button', { hasText: 'Apply Leave' });
    await expect(applyButton).toBeVisible();
  });
});
