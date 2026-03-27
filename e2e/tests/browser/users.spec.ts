import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Users Management', () => {
  test('page loads at /users', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/users');
    await page.waitForLoadState('networkidle');

    // The UsersView is wrapped in an NCard with the title from i18n userMgmt.title
    const card = page.locator('.n-card');
    await expect(card.first()).toBeVisible({ timeout: 15_000 });
  });

  test('user list table is visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/users');
    await page.waitForLoadState('networkidle');

    // NDataTable renders a .n-data-table element
    const dataTable = page.locator('.n-data-table');
    await expect(dataTable).toBeVisible({ timeout: 15_000 });

    // Table should have header columns (name, email, role, status, etc.)
    const headerCells = page.locator('.n-data-table th');
    const headerCount = await headerCells.count();
    expect(headerCount).toBeGreaterThanOrEqual(4);
  });
});
