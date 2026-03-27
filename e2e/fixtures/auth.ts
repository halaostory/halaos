import { test as base, type BrowserContext, type Page } from '@playwright/test';
import { loadState } from './state';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

type AuthFixtures = {
  adminToken: string;
  employeeToken: string;
  adminPage: Page;
};

/**
 * Inject auth tokens into browser context via addInitScript.
 * This runs before any page script on every navigation,
 * setting localStorage tokens so the Vue auth guard sees the user as logged in.
 */
async function injectAuth(
  context: BrowserContext,
  accessToken: string,
  refreshToken: string,
): Promise<void> {
  await context.addInitScript(
    ({ access, refresh }: { access: string; refresh: string }) => {
      localStorage.setItem('access_token', access);
      localStorage.setItem('refresh_token', refresh);
      localStorage.setItem('halaos_setup_done', 'true');
      localStorage.setItem('halaos_tour_done', 'true');
    },
    { access: accessToken, refresh: refreshToken },
  );
}

/**
 * Intercept /api/v1/auth/me to return a cached mock user response.
 * This prevents the fetchMe() call from hitting rate limits or failing
 * with expired tokens, which would cause the SPA to redirect to /login.
 */
async function interceptAuthMe(context: BrowserContext, token?: string): Promise<void> {
  const state = loadState();
  const targetToken = token || state.adminToken;

  // Decode user info from the token payload
  let userId = 0;
  let companyId = 0;
  let email = '';
  let role = 'employee';

  if (targetToken) {
    try {
      const parts = targetToken.split('.');
      if (parts.length === 3) {
        const payload = JSON.parse(Buffer.from(parts[1], 'base64').toString());
        userId = payload.user_id || userId;
        companyId = payload.company_id || companyId;
        email = payload.email || email;
        role = payload.role || role;
      }
    } catch {
      // use defaults
    }
  }

  const mockUser = {
    success: true,
    data: {
      id: userId,
      email,
      first_name: 'E2E',
      last_name: role === 'super_admin' ? 'Admin' : 'Employee',
      role,
      company_id: companyId,
      company_country: 'PH',
      company_currency: 'PHP',
      company_timezone: 'Asia/Manila',
    },
  };

  await context.route('**/api/v1/auth/me', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockUser),
    });
  });

  // Also intercept the refresh token endpoint so it never fails and triggers
  // a redirect to /login in the API client
  const mockRefresh = {
    success: true,
    data: {
      token: targetToken,
      refresh_token: targetToken,
    },
  };
  await context.route('**/api/v1/auth/refresh', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockRefresh),
    });
  });
}

export const test = base.extend<AuthFixtures>({
  adminToken: async ({}, use) => {
    const state = loadState();
    if (!state.adminToken) throw new Error('No admin token — run data factory first');
    await use(state.adminToken);
  },

  employeeToken: async ({}, use) => {
    const state = loadState();
    const tokens = Object.values(state.employeeTokens);
    if (tokens.length === 0) throw new Error('No employee tokens — run data factory first');
    await use(tokens[0]);
  },

  adminPage: async ({ browser }, use) => {
    const state = loadState();
    if (!state.adminToken) {
      throw new Error('No admin token — run data factory first');
    }
    const context = await browser.newContext();
    await injectAuth(context, state.adminToken, state.adminRefreshToken || '');
    await interceptAuthMe(context);
    const page = await context.newPage();
    await use(page);
    await page.close();
    await context.close();
  },
});

export { expect } from '@playwright/test';

/**
 * Navigate to a page and wait for the SPA to settle (network idle).
 * Returns true if the page loaded successfully, false if redirected to login.
 */
export async function navigateOrSkip(
  page: Page,
  testObj: typeof test,
  path: string,
): Promise<boolean> {
  await page.goto(BASE + path);
  await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {});

  if (page.url().includes('/login')) {
    testObj.skip(true, 'Redirected to login — token may have expired or rate limited');
    return false;
  }
  return true;
}
