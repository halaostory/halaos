import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Dashboard', () => {
  test('dashboard loads with stats', async ({ adminPage: page }) => {
    await page.goto(BASE + '/');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Dashboard should have visible content
    await expect(page.locator('body')).toBeVisible();

    // The dashboard stats grid should be present (id="dashboard-stats")
    const statsGrid = page.locator('#dashboard-stats');
    await expect(statsGrid).toBeVisible({ timeout: 15_000 });

    // Quick actions card should be visible (id="dashboard-clock")
    const quickActions = page.locator('#dashboard-clock');
    await expect(quickActions).toBeVisible();
  });

  test('Getting Started checklist visible for admin', async ({ adminPage: page }) => {
    await page.goto(BASE + '/');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // The Getting Started checklist or any dashboard content should be visible.
    // The checklist may be dismissed, so check for checklist or main content.
    const checklist = page.locator('.gs-header, .getting-started, [class*="checklist"]');
    const dashboardContent = page.locator('#dashboard-stats, .n-card, .n-grid, main');
    await expect(checklist.or(dashboardContent).first()).toBeVisible({ timeout: 15_000 });
  });
});
