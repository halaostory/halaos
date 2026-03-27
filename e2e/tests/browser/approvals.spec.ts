import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Approvals', () => {
  test('page loads', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/approvals');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toContainText('Approvals');
  });

  test('approval list or empty state', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/approvals');
    await page.waitForLoadState('networkidle');

    // Either the data table with pending approvals or the empty state should be visible
    const table = page.locator('.n-data-table');
    const emptyState = page.locator('text=All caught up!');
    const hasTable = await table.isVisible().catch(() => false);
    const hasEmpty = await emptyState.isVisible().catch(() => false);

    expect(hasTable || hasEmpty).toBe(true);
  });
});
