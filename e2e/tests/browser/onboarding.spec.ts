import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Onboarding', () => {
  test('page loads without error', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/onboarding');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('heading and tabs visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/onboarding');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h2')).toBeVisible();
    // Tabs: Onboarding, Offboarding
    await expect(page.locator('.n-tabs')).toBeVisible();
  });

  test('action buttons present', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/onboarding');
    await page.waitForLoadState('networkidle');
    // Manage Templates and Start Workflow buttons
    await expect(page.locator('.n-button').first()).toBeVisible();
  });
});
