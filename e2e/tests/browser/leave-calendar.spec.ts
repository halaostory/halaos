import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Leave Calendar', () => {
  test('page loads', async ({ adminPage: page }) => {
    await page.goto(BASE + '/leave-calendar');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('h2')).toContainText('Leave Calendar');
  });

  test('calendar component renders', async ({ adminPage: page }) => {
    await page.goto(BASE + '/leave-calendar');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

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
