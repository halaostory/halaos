import { test, expect } from '@playwright/test';
import { loadState } from '../../fixtures/state';
import { createApiClient } from '../../fixtures/api-client';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Leave API', () => {
  let adminApi: Awaited<ReturnType<typeof createApiClient>>;
  let empApi: Awaited<ReturnType<typeof createApiClient>>;
  let state: ReturnType<typeof loadState>;

  test.beforeAll(async () => {
    state = loadState();
    adminApi = await createApiClient(BASE, state.adminToken);
    const tokens = Object.values(state.employeeTokens);
    const empToken = tokens[0];
    empApi = await createApiClient(BASE, empToken);
  });

  test.afterAll(async () => {
    await adminApi.dispose();
    await empApi.dispose();
  });

  // ── Leave Types ──────────────────────────────────────────────

  test.describe('Leave Types', () => {
    test('GET /api/v1/leaves/types - list leave types', async () => {
      const data = await adminApi.get('/api/v1/leaves/types');
      expect(Array.isArray(data)).toBe(true);
    });

    test('POST /api/v1/leaves/types - create leave type (admin)', async () => {
      const uniqueCode = `E2E_LT_${Date.now()}`;
      const data = await adminApi.post('/api/v1/leaves/types', {
        code: uniqueCode,
        name: `E2E Leave ${uniqueCode}`,
        is_paid: true,
        default_days: '15',
        min_days_notice: 3,
        accrual_type: 'annual',
        is_statutory: false,
      });
      expect(data).toBeTruthy();
      expect(data.id).toBeTruthy();
      expect(data.code).toBe(uniqueCode);
    });

    test('POST /api/v1/leaves/types - missing required fields returns error', async () => {
      await expect(
        adminApi.post('/api/v1/leaves/types', { is_paid: true })
      ).rejects.toThrow();
    });

    test('POST /api/v1/leaves/types - employee cannot create leave type', async () => {
      await expect(
        empApi.post('/api/v1/leaves/types', {
          code: 'EMP_ATTEMPT',
          name: 'Employee Attempt',
        })
      ).rejects.toThrow();
    });
  });

  // ── Leave Balances ───────────────────────────────────────────

  test.describe('Leave Balances', () => {
    test('GET /api/v1/leaves/balances - get own balances (employee)', async () => {
      const data = await empApi.get('/api/v1/leaves/balances');
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/leaves/balances - admin gets balances', async () => {
      const data = await adminApi.get('/api/v1/leaves/balances');
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/leaves/balances/all - admin gets all balances', async () => {
      const data = await adminApi.get('/api/v1/leaves/balances/all');
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/leaves/balances/all - with year param', async () => {
      const year = new Date().getFullYear().toString();
      const data = await adminApi.get('/api/v1/leaves/balances/all', { year });
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/leaves/balances/all - employee cannot access all balances', async () => {
      await expect(
        empApi.get('/api/v1/leaves/balances/all')
      ).rejects.toThrow();
    });
  });

  // ── Leave Requests ───────────────────────────────────────────

  test.describe('Leave Requests', () => {
    let createdRequestId: number | null = null;

    test('POST /api/v1/leaves/requests - create leave request (employee)', async () => {
      const leaveTypeId = state.leaveTypeIds?.[0];
      test.skip(!leaveTypeId, 'No leave type IDs in state');

      const startDate = '2026-06-01';
      const endDate = '2026-06-02';
      const data = await empApi.post('/api/v1/leaves/requests', {
        leave_type_id: leaveTypeId,
        start_date: startDate,
        end_date: endDate,
        days: '2',
        reason: 'E2E test leave request',
      });
      expect(data).toBeTruthy();
      expect(data.id).toBeTruthy();
      createdRequestId = data.id;
    });

    test('POST /api/v1/leaves/requests - missing required fields returns error', async () => {
      await expect(
        empApi.post('/api/v1/leaves/requests', {
          start_date: '2026-06-01',
        })
      ).rejects.toThrow();
    });

    test('POST /api/v1/leaves/requests - invalid date format returns error', async () => {
      const leaveTypeId = state.leaveTypeIds?.[0];
      test.skip(!leaveTypeId, 'No leave type IDs in state');

      await expect(
        empApi.post('/api/v1/leaves/requests', {
          leave_type_id: leaveTypeId,
          start_date: '06/01/2026',
          end_date: '06/02/2026',
          days: '2',
        })
      ).rejects.toThrow();
    });

    test('GET /api/v1/leaves/requests - list leave requests', async () => {
      const data = await adminApi.get('/api/v1/leaves/requests');
      expect(data).toBeTruthy();
    });

    test('GET /api/v1/leaves/requests - filter by status', async () => {
      const data = await adminApi.get('/api/v1/leaves/requests', {
        status: 'pending',
      });
      expect(data).toBeTruthy();
    });

    test('GET /api/v1/leaves/requests - with pagination', async () => {
      const data = await adminApi.get('/api/v1/leaves/requests', {
        page: '1',
        limit: '5',
      });
      expect(data).toBeTruthy();
    });
  });

  // ── Leave Calendar ───────────────────────────────────────────

  test.describe('Leave Calendar', () => {
    test('GET /api/v1/leaves/calendar - get calendar with date range', async () => {
      const data = await adminApi.get('/api/v1/leaves/calendar', {
        start: '2026-01-01',
        end: '2026-12-31',
      });
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/leaves/calendar - missing dates returns error', async () => {
      await expect(
        adminApi.get('/api/v1/leaves/calendar')
      ).rejects.toThrow();
    });

    test('GET /api/v1/leaves/calendar - invalid date format returns error', async () => {
      await expect(
        adminApi.get('/api/v1/leaves/calendar', {
          start: '01-01-2026',
          end: '12-31-2026',
        })
      ).rejects.toThrow();
    });
  });

  // ── Unauthenticated access ───────────────────────────────────

  test.describe('Unauthenticated', () => {
    test('GET /api/v1/leaves/types - no token returns 401', async () => {
      const noAuthApi = await createApiClient(BASE);
      try {
        await expect(
          noAuthApi.get('/api/v1/leaves/types')
        ).rejects.toThrow();
      } finally {
        await noAuthApi.dispose();
      }
    });
  });
});
