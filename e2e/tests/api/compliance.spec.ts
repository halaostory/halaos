import { test, expect } from '@playwright/test';
import { loadState } from '../../fixtures/state';
import { createApiClient } from '../../fixtures/api-client';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Compliance & Tax API', () => {
  let api: Awaited<ReturnType<typeof createApiClient>>;

  test.beforeAll(async () => {
    const state = loadState();
    api = await createApiClient(BASE, state.adminToken);
  });

  test.afterAll(async () => {
    await api.dispose();
  });

  // ---- SSS Table ----

  test('GET /compliance/sss-table returns SSS contribution brackets', async () => {
    const data = await api.get('/api/v1/compliance/sss-table');
    expect(Array.isArray(data)).toBe(true);
    expect(data.length).toBeGreaterThanOrEqual(20);
  });

  test('GET /compliance/sss-table with as_of param', async () => {
    const data = await api.get('/api/v1/compliance/sss-table', { as_of: '2025-01-01' });
    expect(Array.isArray(data)).toBe(true);
  });

  // ---- PhilHealth Table ----

  test('GET /compliance/philhealth-table returns PhilHealth rates', async () => {
    const data = await api.get('/api/v1/compliance/philhealth-table');
    expect(Array.isArray(data)).toBe(true);
    expect(data.length).toBeGreaterThan(0);
  });

  // ---- Pag-IBIG Table ----

  test('GET /compliance/pagibig-table returns Pag-IBIG rates', async () => {
    const data = await api.get('/api/v1/compliance/pagibig-table');
    expect(Array.isArray(data)).toBe(true);
    expect(data.length).toBeGreaterThan(0);
  });

  // ---- BIR Tax Table ----

  test('GET /compliance/bir-tax-table returns semi-monthly brackets by default', async () => {
    const data = await api.get('/api/v1/compliance/bir-tax-table');
    expect(Array.isArray(data)).toBe(true);
    expect(data.length).toBeGreaterThan(0);
  });

  test('GET /compliance/bir-tax-table with frequency=monthly', async () => {
    const data = await api.get('/api/v1/compliance/bir-tax-table', { frequency: 'monthly' });
    expect(Array.isArray(data)).toBe(true);
    expect(data.length).toBeGreaterThan(0);
  });

  // ---- Government Forms ----

  test('GET /compliance/government-forms lists forms', async () => {
    const data = await api.get('/api/v1/compliance/government-forms');
    // May be an array or wrapped object depending on pagination
    expect(data).toBeDefined();
  });

  // ---- Tax Filings ----

  test('GET /tax-filings lists tax filings', async () => {
    const data = await api.get('/api/v1/tax-filings');
    expect(data).toBeDefined();
    // Response wraps {data, summary, year}
    expect(data).toHaveProperty('year');
  });

  test('GET /tax-filings with year filter', async () => {
    const data = await api.get('/api/v1/tax-filings', { year: '2025' });
    expect(data).toBeDefined();
    expect(data.year).toBe(2025);
  });

  test('GET /tax-filings/overdue returns overdue filings', async () => {
    const data = await api.get('/api/v1/tax-filings/overdue');
    expect(Array.isArray(data)).toBe(true);
  });

  test('GET /tax-filings/upcoming returns upcoming filings', async () => {
    const data = await api.get('/api/v1/tax-filings/upcoming');
    expect(Array.isArray(data)).toBe(true);
  });

  // ---- Salary Structures ----

  test('GET /salary/structures lists salary structures', async () => {
    const data = await api.get('/api/v1/salary/structures');
    expect(Array.isArray(data)).toBe(true);
  });

  // ---- Salary Components ----

  test('GET /salary/components lists salary components', async () => {
    const data = await api.get('/api/v1/salary/components');
    expect(Array.isArray(data)).toBe(true);
  });

  // ---- Permission: unauthenticated access denied ----

  test('compliance endpoints reject unauthenticated requests', async () => {
    const noAuth = await createApiClient(BASE);
    try {
      await expect(noAuth.get('/api/v1/compliance/sss-table')).rejects.toThrow(/API error/);
    } finally {
      await noAuth.dispose();
    }
  });

  // ---- Error: invalid frequency for BIR tax table still returns data ----

  test('GET /compliance/bir-tax-table with invalid frequency returns empty or data', async () => {
    // The server defaults to "semi_monthly" if unrecognized, so it may still return data
    const data = await api.get('/api/v1/compliance/bir-tax-table', { frequency: 'invalid_freq' });
    expect(Array.isArray(data)).toBe(true);
  });
});
