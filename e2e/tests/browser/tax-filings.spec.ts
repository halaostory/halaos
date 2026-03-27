import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Tax Filings', () => {
  test('page loads without error', async ({ adminPage: page }) => {
    await page.goto(BASE + '/tax-filings');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('body')).toBeVisible();
    await expect(page.locator('text=404')).not.toBeVisible();
  });

  test('heading and year selector visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/tax-filings');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    await expect(page.locator('h2')).toBeVisible({ timeout: 15_000 });
    // Year input number selector or tabs should be visible
    const yearInput = page.locator('.n-input-number');
    const tabs = page.locator('.n-tabs');
    await expect(yearInput.or(tabs).first()).toBeVisible({ timeout: 15_000 });
  });

  test('tabs and data table visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/tax-filings');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Tabs: All Filings, Overdue, Upcoming — or empty state if no data
    const tabs = page.locator('.n-tabs');
    const emptyState = page.locator('.n-empty');
    await expect(tabs.or(emptyState).first()).toBeVisible({ timeout: 15_000 });
    const table = page.locator('.n-data-table');
    await expect(table.or(emptyState).first()).toBeVisible({ timeout: 15_000 });
  });
});
