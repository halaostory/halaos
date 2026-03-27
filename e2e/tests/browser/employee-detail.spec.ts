import { test, expect } from '../../fixtures/auth';
import { loadState } from '../../fixtures/state';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Employee Detail', () => {
  test('page shows employee info', async ({ adminContext }) => {
    const state = loadState();
    const employeeId = state.employeeIds[0];
    if (!employeeId) {
      test.skip(true, 'No employee IDs in test state');
      return;
    }

    const page = await adminContext.newPage();
    await page.goto(BASE + '/employees/' + employeeId);
    await page.waitForLoadState('networkidle');

    // The page shows the employee name as an h2 element
    const heading = page.locator('h2');
    await expect(heading).toBeVisible({ timeout: 15_000 });

    // Basic Info card should be visible (NCard with title from i18n employee.basicInfo)
    const basicInfoCard = page.locator('.n-card').first();
    await expect(basicInfoCard).toBeVisible();

    // The NDescriptions component should show employee details
    const descriptions = page.locator('.n-descriptions');
    await expect(descriptions.first()).toBeVisible();
  });

  test('tabs/sections are visible (basic info, salary, documents, timeline)', async ({ adminContext }) => {
    const state = loadState();
    const employeeId = state.employeeIds[0];
    if (!employeeId) {
      test.skip(true, 'No employee IDs in test state');
      return;
    }

    const page = await adminContext.newPage();
    await page.goto(BASE + '/employees/' + employeeId);
    await page.waitForLoadState('networkidle');

    // Wait for the page to load
    await expect(page.locator('h2')).toBeVisible({ timeout: 15_000 });

    // The detail page uses NCard sections as "tabs" -- each section is an NCard
    // Check for key sections: Basic Info, Salary, Documents, Timeline
    const cards = page.locator('.n-card');
    const cardCount = await cards.count();
    expect(cardCount).toBeGreaterThanOrEqual(3);

    // Salary card should have the assign salary button
    const salaryButton = page.getByRole('button', { name: /salary/i });
    await expect(salaryButton.first()).toBeVisible();

    // Documents section should exist
    const docsSection = page.locator('.n-card').filter({ hasText: /document/i });
    await expect(docsSection.first()).toBeVisible();

    // Timeline section should exist
    const timelineSection = page.locator('.n-card').filter({ hasText: /timeline/i });
    await expect(timelineSection.first()).toBeVisible();
  });
});
