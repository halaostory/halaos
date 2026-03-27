import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('DTR (Daily Time Record)', () => {
  test('page loads', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/dtr');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toContainText('Daily Time Record');
  });

  test('content visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/dtr');
    await page.waitForLoadState('networkidle');

    // Date range picker should be visible
    const datePicker = page.locator('.n-date-picker');
    await expect(datePicker).toBeVisible();

    // Generate button should be visible
    const generateButton = page.locator('button', { hasText: 'Generate' });
    await expect(generateButton).toBeVisible();

    // Data table should be present
    const table = page.locator('.n-data-table');
    await expect(table).toBeVisible();
  });
});
