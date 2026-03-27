import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Schedules', () => {
  test('page loads', async ({ adminPage: page }) => {
    await page.goto(BASE + '/schedules');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Page should show tabs or schedule content
    const tabs = page.locator('.n-tabs');
    const card = page.locator('.n-card');
    await expect(tabs.or(card).first()).toBeVisible({ timeout: 15_000 });
  });

  test('schedule content visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/schedules');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // The tabs component should render with weekly/templates tabs
    const tabs = page.locator('.n-tabs');
    await expect(tabs).toBeVisible({ timeout: 15_000 });

    // Week navigation button or card content should be present
    const weekNav = page.locator('button').filter({ hasText: 'Previous Week' });
    const card = page.locator('.n-card');
    await expect(weekNav.or(card).first()).toBeVisible({ timeout: 15_000 });
  });
});
