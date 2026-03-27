import { test, expect } from '@playwright/test';
import { loadState } from '../../fixtures/state';
import { createApiClient } from '../../fixtures/api-client';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Benefits API', () => {
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

  // ── Benefit Plans ────────────────────────────────────────────

  test.describe('Benefit Plans', () => {
    let createdPlanId: number | null = null;

    test('GET /api/v1/benefits/plans - list benefit plans', async () => {
      const data = await adminApi.get('/api/v1/benefits/plans');
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/benefits/plans - employee can list plans', async () => {
      const data = await empApi.get('/api/v1/benefits/plans');
      expect(Array.isArray(data)).toBe(true);
    });

    test('POST /api/v1/benefits/plans - create benefit plan (admin)', async () => {
      const ts = Date.now();
      const data = await adminApi.post('/api/v1/benefits/plans', {
        name: `E2E Health Plan ${ts}`,
        category: 'health',
        description: 'E2E test health benefit plan',
        provider: 'E2E Insurance Co',
        employer_share: 5000.0,
        employee_share: 1500.0,
        coverage_amount: 500000.0,
        eligibility_type: 'all',
        eligibility_months: 0,
      });
      expect(data).toBeTruthy();
      expect(data.id).toBeTruthy();
      expect(data.name).toContain('E2E Health Plan');
      expect(data.category).toBe('health');
      createdPlanId = data.id;
    });

    test('POST /api/v1/benefits/plans - missing required fields returns error', async () => {
      await expect(
        adminApi.post('/api/v1/benefits/plans', {
          description: 'Missing name and category',
        })
      ).rejects.toThrow();
    });

    test('POST /api/v1/benefits/plans - employee cannot create plan', async () => {
      await expect(
        empApi.post('/api/v1/benefits/plans', {
          name: 'Employee Attempt Plan',
          category: 'health',
        })
      ).rejects.toThrow();
    });

    test('GET /api/v1/benefits/plans/:id - get plan by id', async () => {
      const planId = createdPlanId ?? state.benefitPlanIds?.[0];
      test.skip(!planId, 'No benefit plan ID available');

      const data = await adminApi.get(`/api/v1/benefits/plans/${planId}`);
      expect(data).toBeTruthy();
      expect(data.id).toBe(planId);
    });

    test('GET /api/v1/benefits/plans/:id - non-existent plan returns error', async () => {
      await expect(
        adminApi.get('/api/v1/benefits/plans/999999')
      ).rejects.toThrow();
    });
  });

  // ── Benefit Summary ──────────────────────────────────────────

  test.describe('Benefit Summary', () => {
    test('GET /api/v1/benefits/summary - get benefit summary (admin)', async () => {
      const data = await adminApi.get('/api/v1/benefits/summary');
      expect(data).toBeTruthy();
    });

    test('GET /api/v1/benefits/summary - employee cannot access summary', async () => {
      await expect(
        empApi.get('/api/v1/benefits/summary')
      ).rejects.toThrow();
    });
  });

  // ── Enrollments ──────────────────────────────────────────────

  test.describe('Enrollments', () => {
    let createdEnrollmentId: number | null = null;

    test('GET /api/v1/benefits/enrollments - list enrollments (admin)', async () => {
      const data = await adminApi.get('/api/v1/benefits/enrollments');
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/benefits/enrollments - filter by status', async () => {
      const data = await adminApi.get('/api/v1/benefits/enrollments', {
        status: 'active',
      });
      expect(Array.isArray(data)).toBe(true);
    });

    test('POST /api/v1/benefits/enrollments - create enrollment (admin)', async () => {
      const empId = state.employeeIds?.[0];
      const planId = state.benefitPlanIds?.[0];
      test.skip(!empId || !planId, 'No employee or benefit plan IDs in state');

      const data = await adminApi.post('/api/v1/benefits/enrollments', {
        employee_id: empId,
        plan_id: planId,
        effective_date: '2026-04-01',
        employer_share: 5000.0,
        employee_share: 1500.0,
        notes: 'E2E test enrollment',
      });
      expect(data).toBeTruthy();
      expect(data.id).toBeTruthy();
      createdEnrollmentId = data.id;
    });

    test('POST /api/v1/benefits/enrollments - missing required fields returns error', async () => {
      await expect(
        adminApi.post('/api/v1/benefits/enrollments', {
          notes: 'Missing required fields',
        })
      ).rejects.toThrow();
    });

    test('POST /api/v1/benefits/enrollments - invalid date format returns error', async () => {
      const empId = state.employeeIds?.[0];
      const planId = state.benefitPlanIds?.[0];
      test.skip(!empId || !planId, 'No employee or benefit plan IDs in state');

      await expect(
        adminApi.post('/api/v1/benefits/enrollments', {
          employee_id: empId,
          plan_id: planId,
          effective_date: '04/01/2026',
        })
      ).rejects.toThrow();
    });

    test('POST /api/v1/benefits/enrollments - employee cannot create enrollment', async () => {
      await expect(
        empApi.post('/api/v1/benefits/enrollments', {
          employee_id: 1,
          plan_id: 1,
          effective_date: '2026-04-01',
        })
      ).rejects.toThrow();
    });

    test('GET /api/v1/benefits/my-enrollments - employee can list own enrollments', async () => {
      const data = await empApi.get('/api/v1/benefits/my-enrollments');
      expect(Array.isArray(data)).toBe(true);
    });
  });

  // ── Claims ───────────────────────────────────────────────────

  test.describe('Claims', () => {
    test('GET /api/v1/benefits/claims - list claims (admin)', async () => {
      const data = await adminApi.get('/api/v1/benefits/claims');
      expect(data).toBeTruthy();
      expect(data.items).toBeDefined();
      expect(Array.isArray(data.items)).toBe(true);
    });

    test('GET /api/v1/benefits/claims - filter by status', async () => {
      const data = await adminApi.get('/api/v1/benefits/claims', {
        status: 'pending',
      });
      expect(data).toBeTruthy();
      expect(data.items).toBeDefined();
    });

    test('GET /api/v1/benefits/claims - with pagination', async () => {
      const data = await adminApi.get('/api/v1/benefits/claims', {
        page: '1',
        limit: '5',
      });
      expect(data).toBeTruthy();
      expect(data.items).toBeDefined();
      expect(data.limit).toBe(5);
    });

    test('GET /api/v1/benefits/claims - employee cannot list all claims', async () => {
      await expect(
        empApi.get('/api/v1/benefits/claims')
      ).rejects.toThrow();
    });
  });

  // ── Unauthenticated access ───────────────────────────────────

  test.describe('Unauthenticated', () => {
    test('GET /api/v1/benefits/plans - no token returns 401', async () => {
      const noAuthApi = await createApiClient(BASE);
      try {
        await expect(
          noAuthApi.get('/api/v1/benefits/plans')
        ).rejects.toThrow();
      } finally {
        await noAuthApi.dispose();
      }
    });
  });
});
