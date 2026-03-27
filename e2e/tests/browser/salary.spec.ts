import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Salary Configuration', () => {
  test('page loads without error', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/salary');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('heading and tabs visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/salary');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toBeVisible();
    // Tabs: Structures, Components
    await expect(page.locator('.n-tabs')).toBeVisible();
  });

  test('data table renders', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/salary');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('.n-data-table')).toBeVisible();
  });
});
