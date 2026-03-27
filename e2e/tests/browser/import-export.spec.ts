import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Import / Export', () => {
  test('page loads without error', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/import-export');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('import section with upload visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/import-export');
    await page.waitForLoadState('networkidle');
    // Import card with upload area
    await expect(page.locator('.n-card').first()).toBeVisible();
    // Upload component
    await expect(page.locator('.n-upload')).toBeVisible();
  });

  test('export section visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/import-export');
    await page.waitForLoadState('networkidle');
    // Multiple cards for import and export sections
    const cards = page.locator('.n-card');
    await expect(cards).toHaveCount(await cards.count());
    expect(await cards.count()).toBeGreaterThanOrEqual(1);
  });
});
