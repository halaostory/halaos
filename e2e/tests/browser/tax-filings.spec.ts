import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Tax Filings', () => {
  test('page loads without error', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/tax-filings');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('heading and year selector visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/tax-filings');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toBeVisible();
    // Year input number selector
    await expect(page.locator('.n-input-number')).toBeVisible();
  });

  test('tabs and data table visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/tax-filings');
    await page.waitForLoadState('networkidle');
    // Tabs: All Filings, Overdue, Upcoming
    await expect(page.locator('.n-tabs')).toBeVisible();
    await expect(page.locator('.n-data-table')).toBeVisible();
  });
});
