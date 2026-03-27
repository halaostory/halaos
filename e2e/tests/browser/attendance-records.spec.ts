import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Attendance Records', () => {
  test('page loads', async ({ adminPage: page }) => {
    await page.goto(BASE + '/attendance/records');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('h2')).toContainText('Attendance Records');
  });

  test('date picker visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/attendance/records');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // NDatePicker renders an input with type="daterange"
    const datePicker = page.locator('.n-date-picker');
    await expect(datePicker).toBeVisible();
  });

  test('table or data visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/attendance/records');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // NDataTable renders as .n-data-table
    const table = page.locator('.n-data-table');
    await expect(table).toBeVisible();
  });
});
