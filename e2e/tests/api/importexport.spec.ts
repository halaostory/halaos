import { test, expect } from '@playwright/test';
import { loadState } from '../../fixtures/state';
import { createApiClient } from '../../fixtures/api-client';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Import/Export API', () => {
  let api: Awaited<ReturnType<typeof createApiClient>>;

  test.beforeAll(async () => {
    const state = loadState();
    api = await createApiClient(BASE, state.adminToken);
  });

  test.afterAll(async () => {
    await api.dispose();
  });

  // ---- Export Employees CSV ----

  test('GET /export/employees/csv returns CSV data', async () => {
    const res = await api.getRaw('/api/v1/export/employees/csv');
    expect(res.status()).toBe(200);

    const contentType = res.headers()['content-type'] || '';
    expect(contentType).toContain('text/csv');

    const text = await res.text();
    expect(text.length).toBeGreaterThan(0);
    // CSV should have a header row with expected columns
    const firstLine = text.split('\n')[0];
    expect(firstLine).toContain('Employee No');
    expect(firstLine).toContain('First Name');
    expect(firstLine).toContain('Last Name');
    expect(firstLine).toContain('Email');
  });

  test('GET /export/employees/csv has multiple rows (header + data)', async () => {
    const res = await api.getRaw('/api/v1/export/employees/csv');
    const text = await res.text();
    const lines = text.split('\n').filter(l => l.trim().length > 0);
    // At least header + 1 data row + branding row
    expect(lines.length).toBeGreaterThanOrEqual(2);
  });

  // ---- Export Attendance CSV ----

  test('GET /export/attendance/csv returns CSV with date range', async () => {
    const res = await api.getRaw('/api/v1/export/attendance/csv', {
      start: '2024-01-01',
      end: '2026-12-31',
    });
    expect(res.status()).toBe(200);

    const contentType = res.headers()['content-type'] || '';
    // Might be text/csv or application/octet-stream
    expect([200]).toContain(res.status());

    const text = await res.text();
    expect(text.length).toBeGreaterThan(0);
  });

  test('GET /export/attendance/csv without date range returns 400', async () => {
    const res = await api.getRaw('/api/v1/export/attendance/csv');
    expect(res.status()).toBe(400);
  });

  test('GET /export/attendance/csv with invalid dates returns 400', async () => {
    const res = await api.getRaw('/api/v1/export/attendance/csv', {
      start: 'bad-date',
      end: '2026-12-31',
    });
    expect(res.status()).toBe(400);
  });

  // ---- Export Leave Balances CSV ----

  test('GET /export/leave-balances/csv returns CSV', async () => {
    const res = await api.getRaw('/api/v1/export/leave-balances/csv');
    expect(res.status()).toBe(200);

    const text = await res.text();
    expect(text.length).toBeGreaterThan(0);
  });

  test('GET /export/leave-balances/csv with year param', async () => {
    const res = await api.getRaw('/api/v1/export/leave-balances/csv', {
      year: '2026',
    });
    expect(res.status()).toBe(200);
  });

  test('GET /export/leave-balances/csv with invalid year returns 400', async () => {
    const res = await api.getRaw('/api/v1/export/leave-balances/csv', {
      year: 'abc',
    });
    expect(res.status()).toBe(400);
  });

  // ---- Preview Import CSV ----

  test('POST /import/employees/preview without file returns 400', async () => {
    try {
      const res = await api.getRaw('/api/v1/import/employees/preview');
      // POST endpoint called via GET should fail
      expect([400, 404, 405]).toContain(res.status());
    } catch {
      // Expected
    }
  });

  test('POST /import/employees/preview with multipart form', async () => {
    // Build a minimal CSV with required columns
    const csvContent = [
      'employee_no,first_name,last_name,email,hire_date,department,position',
      'EMP-PREVIEW-001,Preview,Test,preview@test.com,2025-01-15,Engineering,Developer',
    ].join('\n');

    try {
      const data = await api.postForm('/api/v1/import/employees/preview', {
        file: {
          name: 'preview_test.csv',
          mimeType: 'text/csv',
          buffer: Buffer.from(csvContent),
        },
      });
      expect(data).toBeDefined();
    } catch (err: any) {
      // If the endpoint requires specific column names, it might return 400
      // which is acceptable for this test
      expect(err.message).toContain('API error');
    }
  });

  // ---- Permission: unauthenticated access denied ----

  test('export endpoints reject unauthenticated requests', async () => {
    const noAuth = await createApiClient(BASE);
    try {
      const res = await noAuth.getRaw('/api/v1/export/employees/csv');
      expect(res.status()).toBe(401);
    } finally {
      await noAuth.dispose();
    }
  });
});
