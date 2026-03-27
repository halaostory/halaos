import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Audit', () => {
  test('page loads and content is visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    const response = await page.goto(BASE + '/audit');
    expect(response?.status()).not.toBe(404);
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).toBeVisible();
  });
});
