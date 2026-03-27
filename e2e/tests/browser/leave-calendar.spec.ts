import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Leave Calendar', () => {
  test('page loads', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/leave-calendar');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toContainText('Leave Calendar');
  });

  test('calendar component renders', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/leave-calendar');
    await page.waitForLoadState('networkidle');

    // The calendar renders as a <table> with class "cal-table"
    const calendarTable = page.locator('table.cal-table');
    await expect(calendarTable).toBeVisible();

    // Navigation buttons should be present
    const prevButton = page.locator('button', { hasText: 'Previous Month' });
    const nextButton = page.locator('button', { hasText: 'Next Month' });
    await expect(prevButton).toBeVisible();
    await expect(nextButton).toBeVisible();
  });
});
