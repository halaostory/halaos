import { test, expect } from '@playwright/test';
import { loadState } from '../../fixtures/state';
import { createApiClient } from '../../fixtures/api-client';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Payroll API', () => {
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

  // ── Payroll Cycles ───────────────────────────────────────────

  test.describe('Payroll Cycles', () => {
    let createdCycleId: number | null = null;

    test('GET /api/v1/payroll/cycles - list payroll cycles', async () => {
      const data = await adminApi.get('/api/v1/payroll/cycles');
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/payroll/cycles - with pagination', async () => {
      const data = await adminApi.get('/api/v1/payroll/cycles', {
        page: '1',
        limit: '5',
      });
      expect(Array.isArray(data)).toBe(true);
    });

    test('POST /api/v1/payroll/cycles - create payroll cycle', async () => {
      const ts = Date.now();
      const data = await adminApi.post('/api/v1/payroll/cycles', {
        name: `E2E Cycle ${ts}`,
        period_start: '2026-03-01',
        period_end: '2026-03-15',
        pay_date: '2026-03-20',
        cycle_type: 'regular',
      });
      expect(data).toBeTruthy();
      expect(data.id).toBeTruthy();
      expect(data.name).toContain('E2E Cycle');
      createdCycleId = data.id;
    });

    test('POST /api/v1/payroll/cycles - missing required fields returns error', async () => {
      await expect(
        adminApi.post('/api/v1/payroll/cycles', {
          name: 'Missing dates',
        })
      ).rejects.toThrow();
    });

    test('POST /api/v1/payroll/cycles - invalid date format returns error', async () => {
      await expect(
        adminApi.post('/api/v1/payroll/cycles', {
          name: 'Bad dates',
          period_start: '03-01-2026',
          period_end: '03-15-2026',
          pay_date: '03-20-2026',
        })
      ).rejects.toThrow();
    });

    test('GET /api/v1/payroll/cycles - employee cannot access payroll cycles', async () => {
      await expect(
        empApi.get('/api/v1/payroll/cycles')
      ).rejects.toThrow();
    });
  });

  // ── Payroll Runs ─────────────────────────────────────────────

  test.describe('Payroll Runs', () => {
    test('POST /api/v1/payroll/runs - create payroll run', async () => {
      const cycleId = state.payrollCycleId;
      test.skip(!cycleId, 'No payroll cycle ID in state');

      const data = await adminApi.post('/api/v1/payroll/runs', {
        cycle_id: cycleId,
        run_type: 'simulation',
      });
      expect(data).toBeTruthy();
      expect(data.id).toBeTruthy();
    });

    test('POST /api/v1/payroll/runs - missing cycle_id returns error', async () => {
      await expect(
        adminApi.post('/api/v1/payroll/runs', {
          run_type: 'regular',
        })
      ).rejects.toThrow();
    });

    test('POST /api/v1/payroll/runs - employee cannot create payroll run', async () => {
      await expect(
        empApi.post('/api/v1/payroll/runs', {
          cycle_id: 1,
          run_type: 'regular',
        })
      ).rejects.toThrow();
    });
  });

  // ── Payroll Items ────────────────────────────────────────────

  test.describe('Payroll Items', () => {
    test('GET /api/v1/payroll/cycles/:id/items - list cycle items', async () => {
      const cycleId = state.payrollCycleId;
      test.skip(!cycleId, 'No payroll cycle ID in state');

      const data = await adminApi.get(`/api/v1/payroll/cycles/${cycleId}/items`);
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/payroll/runs/:id/items - invalid ID returns error', async () => {
      await expect(
        adminApi.get('/api/v1/payroll/runs/999999/items')
      ).rejects.toThrow();
    });

    test('GET /api/v1/payroll/cycles/:id/items - employee cannot access', async () => {
      const cycleId = state.payrollCycleId ?? 1;
      await expect(
        empApi.get(`/api/v1/payroll/cycles/${cycleId}/items`)
      ).rejects.toThrow();
    });
  });

  // ── Payslips ─────────────────────────────────────────────────

  test.describe('Payslips', () => {
    test('GET /api/v1/payroll/payslips - list own payslips (employee)', async () => {
      const data = await empApi.get('/api/v1/payroll/payslips');
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/payroll/payslips - admin gets payslips', async () => {
      const data = await adminApi.get('/api/v1/payroll/payslips');
      expect(Array.isArray(data)).toBe(true);
    });
  });

  // ── 13th Month ───────────────────────────────────────────────

  test.describe('13th Month', () => {
    test('GET /api/v1/payroll/13th-month - list 13th month data', async () => {
      const data = await adminApi.get('/api/v1/payroll/13th-month');
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/payroll/13th-month - with year param', async () => {
      const year = new Date().getFullYear().toString();
      const data = await adminApi.get('/api/v1/payroll/13th-month', { year });
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/payroll/13th-month - employee cannot access', async () => {
      await expect(
        empApi.get('/api/v1/payroll/13th-month')
      ).rejects.toThrow();
    });
  });

  // ── Auto Config ──────────────────────────────────────────────

  test.describe('Auto Config', () => {
    test('GET /api/v1/payroll/auto-config - get auto config', async () => {
      const data = await adminApi.get('/api/v1/payroll/auto-config');
      expect(data).toBeTruthy();
      expect(typeof data.auto_run_enabled).toBe('boolean');
    });

    test('GET /api/v1/payroll/auto-config - employee cannot access', async () => {
      await expect(
        empApi.get('/api/v1/payroll/auto-config')
      ).rejects.toThrow();
    });
  });

  // ── Bonus Structures ────────────────────────────────────────

  test.describe('Bonus Structures', () => {
    test('GET /api/v1/payroll/bonus/structures - list bonus structures', async () => {
      const data = await adminApi.get('/api/v1/payroll/bonus/structures');
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/payroll/bonus/structures - filter by status', async () => {
      const data = await adminApi.get('/api/v1/payroll/bonus/structures', {
        status: 'draft',
      });
      expect(Array.isArray(data)).toBe(true);
    });

    test('POST /api/v1/payroll/bonus/structures - create bonus structure', async () => {
      const ts = Date.now();
      const data = await adminApi.post('/api/v1/payroll/bonus/structures', {
        name: `E2E Bonus ${ts}`,
        description: 'E2E test bonus structure',
        bonus_type: 'kpi',
        base_amount: 10000,
        base_type: 'fixed',
        rating_map: { '1': 0.5, '2': 0.75, '3': 1.0, '4': 1.25, '5': 1.5 },
        is_taxable: true,
      });
      expect(data).toBeTruthy();
      expect(data.id).toBeTruthy();
    });

    test('POST /api/v1/payroll/bonus/structures - missing name returns error', async () => {
      await expect(
        adminApi.post('/api/v1/payroll/bonus/structures', {
          bonus_type: 'kpi',
        })
      ).rejects.toThrow();
    });

    test('GET /api/v1/payroll/bonus/structures - employee cannot access', async () => {
      await expect(
        empApi.get('/api/v1/payroll/bonus/structures')
      ).rejects.toThrow();
    });
  });

  // ── Benefit Deductions ───────────────────────────────────────

  test.describe('Benefit Deductions', () => {
    test('GET /api/v1/payroll/benefit-deductions - list benefit deductions', async () => {
      const data = await adminApi.get('/api/v1/payroll/benefit-deductions');
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/payroll/benefit-deductions - filter by employee_id', async () => {
      const empId = state.employeeIds?.[0];
      test.skip(!empId, 'No employee IDs in state');

      const data = await adminApi.get('/api/v1/payroll/benefit-deductions', {
        employee_id: empId.toString(),
      });
      expect(Array.isArray(data)).toBe(true);
    });

    test('GET /api/v1/payroll/benefit-deductions - employee cannot access', async () => {
      await expect(
        empApi.get('/api/v1/payroll/benefit-deductions')
      ).rejects.toThrow();
    });
  });

  // ── Unauthenticated access ───────────────────────────────────

  test.describe('Unauthenticated', () => {
    test('GET /api/v1/payroll/cycles - no token returns 401', async () => {
      const noAuthApi = await createApiClient(BASE);
      try {
        await expect(
          noAuthApi.get('/api/v1/payroll/cycles')
        ).rejects.toThrow();
      } finally {
        await noAuthApi.dispose();
      }
    });
  });
});
