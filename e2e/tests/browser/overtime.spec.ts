import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Overtime', () => {
  test('page loads', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/overtime');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toContainText('Overtime');
  });

  test('apply overtime button visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/overtime');
    await page.waitForLoadState('networkidle');

    const applyButton = page.locator('button', { hasText: 'Apply Overtime' });
    await expect(applyButton).toBeVisible();
  });
});
