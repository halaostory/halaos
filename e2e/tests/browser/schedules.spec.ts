import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Schedules', () => {
  test('page loads', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/schedules');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).toContainText('Shift Schedules');
  });

  test('schedule content visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/schedules');
    await page.waitForLoadState('networkidle');

    // The tabs component should render with weekly/templates tabs
    const tabs = page.locator('.n-tabs');
    await expect(tabs).toBeVisible();

    // Either the schedule data table, empty state, or week navigation should be present
    const weekNav = page.locator('button', { hasText: 'Previous Week' });
    const emptyState = page.locator('text=No schedules yet');
    const table = page.locator('.n-data-table');
    const hasWeekNav = await weekNav.isVisible().catch(() => false);
    const hasEmpty = await emptyState.isVisible().catch(() => false);
    const hasTable = await table.isVisible().catch(() => false);

    expect(hasWeekNav || hasEmpty || hasTable).toBe(true);
  });
});
