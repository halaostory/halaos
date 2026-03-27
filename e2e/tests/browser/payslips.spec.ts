import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Payslips', () => {
  test('page loads', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/payslips');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toContainText('My Payslips');
  });

  test('payslip list or empty state visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/payslips');
    await page.waitForLoadState('networkidle');

    // Either a data table with payslip records or the page heading should be rendered
    const table = page.locator('.n-data-table');
    const emptyText = page.locator('text=No payslips available');
    const hasTable = await table.isVisible().catch(() => false);
    const hasEmpty = await emptyText.isVisible().catch(() => false);

    // At minimum, the table component is always rendered (even if empty)
    expect(hasTable || hasEmpty).toBe(true);
  });
});
