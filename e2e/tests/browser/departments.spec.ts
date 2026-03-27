import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Departments', () => {
  test('page loads at /departments', async ({ adminPage: page }) => {
    await page.goto(BASE + '/departments');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Page heading should be visible
    const heading = page.locator('h2');
    await expect(heading).toBeVisible({ timeout: 15_000 });
  });

  test('department table is visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/departments');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // NDataTable renders a .n-data-table element
    const dataTable = page.locator('.n-data-table');
    await expect(dataTable).toBeVisible({ timeout: 15_000 });
  });

  test('add department button visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/departments');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // The Create button is an NButton with type="primary"
    const createButton = page.locator('button.n-button--primary-type');
    await expect(createButton).toBeVisible({ timeout: 15_000 });
  });
});
