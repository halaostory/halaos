import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('H5 Leave', () => {
  test('leave page loads', async ({ employeeContext }) => {
    const page = await employeeContext.newPage();
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto(BASE + '/m/leave');
    await page.waitForLoadState('load');
    await expect(page.locator('body')).toBeVisible();
  });
});
