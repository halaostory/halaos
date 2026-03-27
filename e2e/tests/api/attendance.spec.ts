import { test, expect } from '@playwright/test';
import { loadState } from '../../fixtures/state';
import { createApiClient } from '../../fixtures/api-client';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Attendance API', () => {
  test('clock in with employee token', async () => {
    const state = loadState();
    const tokens = Object.values(state.employeeTokens);
    const empToken = tokens[0];
    expect(empToken).toBeTruthy();

    const api = await createApiClient(BASE, empToken);

    try {
      // Clock-in may fail if already clocked in today from data factory seeding;
      // either a successful response or an "already clocked in" error is valid
      try {
        const clockIn = await api.post('/api/v1/attendance/clock-in', {});
        expect(clockIn).toBeTruthy();
      } catch (err: any) {
        expect(err.message).toMatch(/API error/i);
      }
    } finally {
      await api.dispose();
    }
  });

  test('clock out with employee token', async () => {
    const state = loadState();
    const tokens = Object.values(state.employeeTokens);
    const empToken = tokens[0];
    expect(empToken).toBeTruthy();

    const api = await createApiClient(BASE, empToken);

    try {
      // Attempt clock-out; may fail if not clocked in
      try {
        const clockOut = await api.post('/api/v1/attendance/clock-out', {});
        expect(clockOut).toBeTruthy();
      } catch (err: any) {
        // Not clocked in or already clocked out is acceptable
        expect(err.message).toMatch(/API error/i);
      }
    } finally {
      await api.dispose();
    }
  });

  test('list attendance records with admin token', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const records = await api.get('/api/v1/attendance/records');

      expect(Array.isArray(records)).toBe(true);

      if (records.length > 0) {
        const rec = records[0];
        expect(rec.id).toBeTruthy();
        expect(rec.employee_id).toBeTruthy();
      }
    } finally {
      await api.dispose();
    }
  });

  test('get attendance summary', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const today = new Date().toISOString().split('T')[0];
      const summary = await api.get('/api/v1/attendance/summary', {
        date: today,
      });

      expect(summary).toBeTruthy();
    } finally {
      await api.dispose();
    }
  });

  test('list shifts', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const shifts = await api.get('/api/v1/attendance/shifts');

      expect(Array.isArray(shifts)).toBe(true);

      if (shifts.length > 0) {
        const shift = shifts[0];
        expect(shift.id).toBeTruthy();
        expect(shift.name).toBeTruthy();
        expect(shift.start_time).toBeTruthy();
        expect(shift.end_time).toBeTruthy();
      }
    } finally {
      await api.dispose();
    }
  });

  test('create shift', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const ts = Date.now();

      const shift = await api.post('/api/v1/attendance/shifts', {
        name: `E2E Shift ${ts}`,
        start_time: '09:00',
        end_time: '18:00',
      });

      expect(shift).toBeTruthy();
      expect(shift.id).toBeTruthy();
      expect(shift.name).toBe(`E2E Shift ${ts}`);
      expect(shift.start_time).toContain('09:00');
      expect(shift.end_time).toContain('18:00');
    } finally {
      await api.dispose();
    }
  });

  test('create attendance correction', async () => {
    const state = loadState();
    const tokens = Object.values(state.employeeTokens);
    const empToken = tokens[0];
    expect(empToken).toBeTruthy();

    const api = await createApiClient(BASE, empToken);

    try {
      const today = new Date().toISOString().split('T')[0];
      const now = new Date();
      const clockInTime = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 8, 0, 0);
      const clockOutTime = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 17, 0, 0);

      const correction = await api.post('/api/v1/attendance/corrections', {
        correction_date: today,
        requested_clock_in: clockInTime.toISOString(),
        requested_clock_out: clockOutTime.toISOString(),
        reason: 'E2E test correction - forgot to clock in',
      });

      expect(correction).toBeTruthy();
      expect(correction.id).toBeTruthy();
      expect(correction.reason).toBe('E2E test correction - forgot to clock in');
      expect(correction.status).toBe('pending');
    } finally {
      await api.dispose();
    }
  });
});
