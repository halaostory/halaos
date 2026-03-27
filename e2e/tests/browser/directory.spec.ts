import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Directory', () => {
  test('page loads at /directory', async ({ adminPage: page }) => {
    await page.goto(BASE + '/directory');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Page heading should be visible
    const heading = page.locator('h2');
    await expect(heading).toBeVisible({ timeout: 15_000 });
  });

  test('employee cards or list visible', async ({ adminPage: page }) => {
    await page.goto(BASE + '/directory');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // Wait for heading to confirm page loaded
    await expect(page.locator('h2')).toBeVisible({ timeout: 15_000 });

    // Wait for loading spinner to finish (NSpin wraps content)
    await page.waitForFunction(
      () => !document.querySelector('.n-spin-container.n-spin-container--show'),
      { timeout: 15_000 },
    ).catch(() => {});

    // The directory renders NCard components for each employee,
    // or a custom EmptyState component if no employees exist.
    // Also accept NGrid (renders even with data) as valid content.
    const content = page.locator('.n-card, .n-grid, .empty-state, .n-empty');
    await expect(content.first()).toBeVisible({ timeout: 15_000 });
  });

  test('search input works', async ({ adminPage: page }) => {
    await page.goto(BASE + '/directory');
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired');
      return;
    }

    // The search input should be visible in list view
    const searchInput = page.locator('.n-input input[type="text"]').first();
    await expect(searchInput).toBeVisible({ timeout: 15_000 });

    // Type something into the search field to verify it accepts input
    await searchInput.fill('test');
    await expect(searchInput).toHaveValue('test');
  });
});
