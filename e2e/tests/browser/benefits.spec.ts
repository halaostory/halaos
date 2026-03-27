import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Benefits', () => {
  test('page loads without error', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/benefits');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).toBeVisible();
    // Should not show 404
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('main heading and tabs visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/benefits');
    await page.waitForLoadState('networkidle');
    // Page heading
    await expect(page.locator('h2')).toBeVisible();
    // Benefits page uses NTabs for My Benefits, Plans, Enrollments, Claims
    await expect(page.locator('.n-tabs')).toBeVisible();
  });

  test('data table renders', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/benefits');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('.n-data-table')).toBeVisible();
  });
});
