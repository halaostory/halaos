import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Attendance Records', () => {
  test('page loads', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/attendance/records');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toContainText('Attendance Records');
  });

  test('date picker visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/attendance/records');
    await page.waitForLoadState('networkidle');

    // NDatePicker renders an input with type="daterange"
    const datePicker = page.locator('.n-date-picker');
    await expect(datePicker).toBeVisible();
  });

  test('table or data visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/attendance/records');
    await page.waitForLoadState('networkidle');

    // NDataTable renders as .n-data-table
    const table = page.locator('.n-data-table');
    await expect(table).toBeVisible();
  });
});
