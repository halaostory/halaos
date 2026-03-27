import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Attendance', () => {
  test('page loads', async ({ adminPage: page }) => {
    await page.goto(BASE + '/attendance');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    // Skip if redirected to login (token expired)
    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('h2')).toContainText('Attendance');
  });

  test('clock in/out buttons or status visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/attendance');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Dismiss onboarding tour if visible
    const tourClose = page.locator('.n-modal .n-button, .n-card-header__close, [aria-label="Close"]');
    if (await tourClose.first().isVisible().catch(() => false)) {
      await tourClose.first().click().catch(() => {});
      await page.waitForTimeout(500);
    }

    // Either the Clock In button or the Clocked In/Out status should be visible
    const clockInButton = page.locator('button', { hasText: 'Clock In' });
    const clockOutButton = page.locator('button', { hasText: 'Clock Out' });

    // Wait for either button to appear (longer timeout for slow renders)
    await expect(clockInButton.or(clockOutButton).first()).toBeVisible({ timeout: 15_000 });
  });
});
