import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Holidays', () => {
  test('page loads without error', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/holidays');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('heading and year selector visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/holidays');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toBeVisible();
    // Year filter select
    await expect(page.locator('.n-select')).toBeVisible();
  });

  test('data table and add button visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/holidays');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('.n-data-table')).toBeVisible();
    // Add Holiday button
    await expect(page.locator('.n-button').first()).toBeVisible();
  });
});
