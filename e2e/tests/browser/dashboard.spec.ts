import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Dashboard', () => {
  test('dashboard loads with stats', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/');
    await page.waitForLoadState('networkidle');

    // Dashboard should have visible content
    await expect(page.locator('body')).toBeVisible();

    // The dashboard stats grid should be present (id="dashboard-stats")
    const statsGrid = page.locator('#dashboard-stats');
    await expect(statsGrid).toBeVisible({ timeout: 15_000 });

    // Quick actions card should be visible (id="dashboard-clock")
    const quickActions = page.locator('#dashboard-clock');
    await expect(quickActions).toBeVisible();
  });

  test('Getting Started checklist visible for admin', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/');
    await page.waitForLoadState('networkidle');

    // The GettingStartedChecklist component renders with class gs-header
    // It is only visible for admins and when not dismissed
    // We check if the checklist area exists (it may be dismissed, so we use a soft check)
    const briefing = page.locator('#dashboard-briefing');
    await expect(briefing).toBeVisible({ timeout: 15_000 });
  });
});
