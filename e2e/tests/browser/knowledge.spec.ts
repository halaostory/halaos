import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Knowledge Base', () => {
  test('page loads without error', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/knowledge');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('card with title and search visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/knowledge');
    await page.waitForLoadState('networkidle');
    // Knowledge base is wrapped in NCard
    await expect(page.locator('.n-card')).toBeVisible();
    // Category filter select
    await expect(page.locator('.n-select')).toBeVisible();
    // Search input
    await expect(page.locator('.n-input')).toBeVisible();
  });

  test('data table renders', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/knowledge');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('.n-data-table')).toBeVisible();
  });
});
