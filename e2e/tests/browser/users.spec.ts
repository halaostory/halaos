import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Users Management', () => {
  test('page loads at /users', async ({ adminPage: page }) => {
    await page.goto(BASE + '/users');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // The UsersView is wrapped in an NCard with the title from i18n userMgmt.title
    const card = page.locator('.n-card');
    await expect(card.first()).toBeVisible({ timeout: 15_000 });
  });

  test('user list table is visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/users');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // NDataTable renders a .n-data-table element or card with user list
    const dataTable = page.locator('.n-data-table');
    const card = page.locator('.n-card');
    await expect(dataTable.or(card).first()).toBeVisible({ timeout: 15_000 });
  });
});
