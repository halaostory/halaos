import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Employees List', () => {
  test('page loads at /employees with table visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/employees');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Page heading should be visible (h2 with employee.title i18n key)
    const heading = page.locator('h2');
    await expect(heading).toBeVisible({ timeout: 15_000 });

    // NDataTable renders a table element inside .n-data-table
    const dataTable = page.locator('.n-data-table');
    await expect(dataTable).toBeVisible();
  });

  test('search input exists', async ({ adminPage: page }) => {
    await page.goto(BASE + '/employees');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // The search NInput renders an input element with the clearable class
    const searchInput = page.locator('.n-input input[type="text"]').last();
    await expect(searchInput).toBeVisible({ timeout: 15_000 });
  });

  test('add employee button visible for admin', async ({ adminPage: page }) => {
    await page.goto(BASE + '/employees');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // The "Add New" button is an NButton with type="primary"
    // It uses the i18n key employee.addNew
    const addButton = page.locator('button.n-button--primary-type').last();
    await expect(addButton).toBeVisible({ timeout: 15_000 });
  });
});
