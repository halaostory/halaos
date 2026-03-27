import { test, expect } from '../../fixtures/auth';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Directory', () => {
  test('page loads at /directory', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/directory');
    await page.waitForLoadState('networkidle');

    // Page heading should be visible
    const heading = page.locator('h2');
    await expect(heading).toBeVisible({ timeout: 15_000 });
  });

  test('employee cards or list visible', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/directory');
    await page.waitForLoadState('networkidle');

    // Wait for loading to finish
    await expect(page.locator('h2')).toBeVisible({ timeout: 15_000 });

    // The directory renders NCard components for each employee in a grid,
    // or an EmptyState component if no employees exist
    const employeeCards = page.locator('.n-card--hoverable');
    const emptyState = page.locator('text=No employees');

    // Either employee cards or the empty state should be present
    const hasCards = await employeeCards.count() > 0;
    const hasEmpty = await emptyState.isVisible().catch(() => false);
    expect(hasCards || hasEmpty).toBeTruthy();
  });

  test('search input works', async ({ adminContext }) => {
    const page = await adminContext.newPage();
    await page.goto(BASE + '/directory');
    await page.waitForLoadState('networkidle');

    // The search input should be visible in list view
    const searchInput = page.locator('.n-input input[type="text"]').first();
    await expect(searchInput).toBeVisible({ timeout: 15_000 });

    // Type something into the search field to verify it accepts input
    await searchInput.fill('test');
    await expect(searchInput).toHaveValue('test');
  });
});
