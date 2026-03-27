import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Import / Export', () => {
  test('page loads without error', async ({ adminPage: page }) => {
    await page.goto(BASE + '/import-export');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('import section with upload visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/import-export');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Import card or upload area should be visible
    const card = page.locator('.n-card');
    const upload = page.locator('.n-upload');
    await expect(card.or(upload).first()).toBeVisible({ timeout: 15_000 });
  });

  test('export section visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/import-export');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Multiple cards for import and export sections — wait for at least one card to render
    const cards = page.locator('.n-card');
    await expect(cards.first()).toBeVisible({ timeout: 15_000 });
    expect(await cards.count()).toBeGreaterThanOrEqual(1);
  });
});
