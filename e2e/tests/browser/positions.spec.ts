import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Positions', () => {
  test('page loads at /positions', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/positions');
    await page.waitForLoadState('networkidle');

    // Page heading should be visible
    const heading = page.locator('h2');
    await expect(heading).toBeVisible({ timeout: 15_000 });
  });

  test('position table is visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/positions');
    await page.waitForLoadState('networkidle');

    // NDataTable renders a .n-data-table element
    const dataTable = page.locator('.n-data-table');
    await expect(dataTable).toBeVisible({ timeout: 15_000 });
  });

  test('create position button visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/positions');
    await page.waitForLoadState('networkidle');

    // The Create button is an NButton with type="primary"
    const createButton = page.locator('button.n-button--primary-type');
    await expect(createButton).toBeVisible({ timeout: 15_000 });
  });
});
