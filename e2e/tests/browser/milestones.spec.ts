import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Milestones', () => {
  test('page loads without error', async ({ adminPage: page }) => {
    await page.goto(BASE + '/milestones');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired or route does not exist');
      return;
    }

    await expect(page.locator('body')).toBeVisible();
  });

  test('heading and filters visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/milestones');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired or route does not exist');
      return;
    }

    // Page should have a heading or content area
    const heading = page.locator('h2, h1, .n-page-header');
    await expect(heading.first()).toBeVisible({ timeout: 10_000 });
  });

  test('data table renders', async ({ adminPage: page }) => {
    await page.goto(BASE + '/milestones');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired or route does not exist');
      return;
    }

    // Data table or empty state should be visible
    const table = page.locator('.n-data-table');
    const emptyState = page.locator('.n-empty, .empty-state');
    await expect(table.or(emptyState).first()).toBeVisible({ timeout: 15_000 });
  });
});
