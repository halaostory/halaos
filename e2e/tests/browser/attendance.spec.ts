import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Attendance', () => {
  test('page loads', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/attendance');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toContainText('Attendance');
  });

  test('clock in/out buttons or status visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/attendance');
    await page.waitForLoadState('networkidle');

    // Either the Clock In button or the Clocked In status tag should be visible
    const clockInButton = page.locator('button', { hasText: 'Clock In' });
    const clockedInTag = page.locator('text=Clocked In');
    const clockOutButton = page.locator('button', { hasText: 'Clock Out' });

    const hasClockIn = await clockInButton.isVisible().catch(() => false);
    const hasClockedIn = await clockedInTag.isVisible().catch(() => false);
    const hasClockOut = await clockOutButton.isVisible().catch(() => false);

    expect(hasClockIn || hasClockedIn || hasClockOut).toBe(true);
  });
});
