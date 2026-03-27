import { test, expect } from '../../fixtures/auth';
import { loadState } from '../../fixtures/state';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

/**
 * Mock employee data for when the API is rate-limited (429).
 * The mock matches the structure expected by EmployeeDetailView.
 */
const MOCK_EMPLOYEE = {
  success: true,
  data: {
    id: 2654,
    company_id: 51,
    user_id: 348,
    employee_no: 'E2E-001',
    first_name: 'Juan',
    last_name: 'Santos',
    email: 'e2e-001@test.halaos.com',
    birth_date: '1989-12-31',
    gender: 'male',
    department_id: 221,
    position_id: 404,
    hire_date: '2023-12-31T00:00:00Z',
    employment_type: 'regular',
    status: 'active',
  },
};

const MOCK_EMPTY = { success: true, data: [] };
const MOCK_EMPTY_OBJ = { success: true, data: {} };

test.describe('Employee Detail', () => {
  test('page shows employee info', async ({ adminPage: page }) => {
    const state = loadState();
    const employeeId = state.employeeIds[0] || 2654;

    // Intercept all employee-related API calls to prevent rate-limit issues
    await page.route(`**/api/v1/employees/${employeeId}`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ...MOCK_EMPLOYEE, data: { ...MOCK_EMPLOYEE.data, id: employeeId } }),
      });
    });
    await page.route(`**/api/v1/employees/${employeeId}/**`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_EMPTY),
      });
    });

    await page.goto(BASE + '/employees/' + employeeId);
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});
    await page.waitForTimeout(500);

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired or rate limited');
      return;
    }

    // The page shows the employee name as an h2 element
    const heading = page.locator('h2').first();
    await expect(heading).toBeVisible({ timeout: 15_000 });

    // Basic Info card should be visible (NCard with title from i18n employee.basicInfo)
    const basicInfoCard = page.locator('.n-card').first();
    await expect(basicInfoCard).toBeVisible({ timeout: 10_000 });

    // The NDescriptions component should show employee details
    const descriptions = page.locator('.n-descriptions');
    await expect(descriptions.first()).toBeVisible({ timeout: 10_000 });
  });

  test('tabs/sections are visible (basic info, salary, documents, timeline)', async ({ adminPage: page }) => {
    const state = loadState();
    const employeeId = state.employeeIds[0] || 2654;

    // Intercept all employee-related API calls to prevent rate-limit issues
    await page.route(`**/api/v1/employees/${employeeId}`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ...MOCK_EMPLOYEE, data: { ...MOCK_EMPLOYEE.data, id: employeeId } }),
      });
    });
    await page.route(`**/api/v1/employees/${employeeId}/**`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_EMPTY),
      });
    });

    await page.goto(BASE + '/employees/' + employeeId);
    await page.waitForLoadState('load', { timeout: 15_000 }).catch(() => {});
    await page.waitForTimeout(500);

    if (page.url().includes('/login')) {
      test.skip(true, 'Redirected to login — token may have expired or rate limited');
      return;
    }

    // Wait for the page to load
    await expect(page.locator('h2')).toBeVisible({ timeout: 15_000 });

    // The detail page uses NCard sections or NTabs — check for multiple cards
    const cards = page.locator('.n-card');
    const tabs = page.locator('.n-tabs');
    const cardCount = await cards.count();
    const hasTabs = await tabs.isVisible().catch(() => false);

    expect(cardCount >= 2 || hasTabs).toBe(true);

    // NDescriptions with employee info should be visible
    const descriptions = page.locator('.n-descriptions');
    await expect(descriptions.first()).toBeVisible({ timeout: 10_000 });
  });
});
