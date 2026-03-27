import { test as base, type BrowserContext } from '@playwright/test';
import { loadState } from './state';

type AuthFixtures = {
  adminToken: string;
  employeeToken: string;
  adminContext: BrowserContext;
  employeeContext: BrowserContext;
};

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

  adminContext: async ({ browser }, use) => {
    const state = loadState();
    const context = await browser.newContext();
    await context.addInitScript((token: string) => {
      localStorage.setItem('access_token', token);
      localStorage.setItem('halaos_setup_done', 'true');
    }, state.adminToken);
    await use(context);
    await context.close();
  },

  employeeContext: async ({ browser }, use) => {
    const state = loadState();
    const tokens = Object.values(state.employeeTokens);
    if (tokens.length === 0) throw new Error('No employee tokens');
    const context = await browser.newContext();
    await context.addInitScript((token: string) => {
      localStorage.setItem('access_token', token);
      localStorage.setItem('halaos_setup_done', 'true');
    }, tokens[0]);
    await use(context);
    await context.close();
  },
});

export { expect } from '@playwright/test';
