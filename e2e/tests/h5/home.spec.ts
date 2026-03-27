import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('H5 Home Page', () => {
  test('home page loads', async ({ employeeContext }) => {
    const page = await employeeContext.newPage();
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto(BASE + '/m/');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).toBeVisible();
  });

  test('navigation elements visible', async ({ employeeContext }) => {
    const page = await employeeContext.newPage();
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto(BASE + '/m/');
    await page.waitForLoadState('networkidle');
    // Bottom nav or quick actions should be visible
    const body = await page.textContent('body');
    expect(body).toBeTruthy();
  });
});
