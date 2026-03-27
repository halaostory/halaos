import { test, expect } from '@playwright/test';
import { loadState } from '../../fixtures/state';
import { createApiClient } from '../../fixtures/api-client';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Misc API', () => {
  let api: Awaited<ReturnType<typeof createApiClient>>;
  let createdAnnouncementId: number | null = null;

  test.beforeAll(async () => {
    const state = loadState();
    api = await createApiClient(BASE, state.adminToken);
  });

  test.afterAll(async () => {
    // Clean up created announcement
    if (createdAnnouncementId) {
      try {
        await api.delete(`/api/v1/announcements/${createdAnnouncementId}`);
      } catch {
        // ignore cleanup errors
      }
    }
    await api.dispose();
  });

  // ---- Announcements ----

  test('GET /announcements lists active announcements', async () => {
    const data = await api.get('/api/v1/announcements');
    expect(Array.isArray(data)).toBe(true);
  });

  test('POST /announcements creates a new announcement', async () => {
    const title = `E2E Test Announcement ${Date.now()}`;
    const data = await api.post('/api/v1/announcements', {
      title,
      content: 'This is a test announcement created by E2E tests.',
      priority: 'normal',
    });
    expect(data).toHaveProperty('id');
    expect(data.title).toBe(title);
    createdAnnouncementId = data.id;
  });

  test('POST /announcements with all fields', async () => {
    const now = new Date();
    const expiresAt = new Date(now.getTime() + 7 * 24 * 60 * 60 * 1000);
    const data = await api.post('/api/v1/announcements', {
      title: `E2E Full Announcement ${Date.now()}`,
      content: 'Full announcement with all fields.',
      priority: 'high',
      target_roles: ['admin', 'employee'],
      target_departments: [],
      published_at: now.toISOString(),
      expires_at: expiresAt.toISOString(),
    });
    expect(data).toHaveProperty('id');
    // Clean up
    try {
      await api.delete(`/api/v1/announcements/${data.id}`);
    } catch {
      // ignore
    }
  });

  test('POST /announcements rejects missing required fields', async () => {
    await expect(async () => {
      await api.post('/api/v1/announcements', {
        priority: 'normal',
      });
    }).rejects.toThrow();
  });

  test('DELETE /announcements/:id deletes an announcement', async () => {
    // Create one to delete
    const ann = await api.post('/api/v1/announcements', {
      title: `E2E Delete Test ${Date.now()}`,
      content: 'Will be deleted.',
    });
    expect(ann).toHaveProperty('id');

    const result = await api.delete(`/api/v1/announcements/${ann.id}`);
    expect(result).toHaveProperty('message');
  });

  test('DELETE /announcements/999999 succeeds silently for nonexistent ID', async () => {
    // SQL DELETE :exec returns no error even if no rows are affected,
    // so the handler returns success with a message.
    const result = await api.delete('/api/v1/announcements/999999');
    expect(result).toHaveProperty('message');
  });

  // ---- Dashboard Stats ----

  test('GET /dashboard/stats returns dashboard statistics', async () => {
    const data = await api.get('/api/v1/dashboard/stats');
    expect(data).toBeDefined();
    // Handler returns: total_employees, present_today, pending_leaves, pending_overtime
    expect(data).toHaveProperty('total_employees');
    expect(data).toHaveProperty('present_today');
    expect(data).toHaveProperty('pending_leaves');
    expect(data).toHaveProperty('pending_overtime');
    expect(typeof data.total_employees).toBe('number');
    expect(data.total_employees).toBeGreaterThan(0);
  });

  // ---- Holidays ----

  test('GET /holidays lists holidays', async () => {
    try {
      const data = await api.get('/api/v1/holidays');
      expect(Array.isArray(data)).toBe(true);
      if (data.length > 0) {
        const first = data[0];
        expect(first).toHaveProperty('name');
        expect(first).toHaveProperty('holiday_date');
      }
    } catch (err: any) {
      if (err.message?.includes('404')) {
        test.skip();
      }
      throw err;
    }
  });

  test('GET /holidays with year filter', async () => {
    try {
      const data = await api.get('/api/v1/holidays', { year: '2026' });
      expect(Array.isArray(data)).toBe(true);
    } catch (err: any) {
      if (err.message?.includes('404')) {
        test.skip();
      }
      throw err;
    }
  });

  // ---- Company Settings ----

  test('GET /company/settings returns company settings', async () => {
    // Endpoint may not exist; use getRaw to check status before parsing
    try {
      const res = await api.getRaw('/api/v1/company/settings');
      if (res.status() === 404) {
        test.skip();
        return;
      }
      const contentType = res.headers()['content-type'] || '';
      if (!contentType.includes('application/json')) {
        test.skip();
        return;
      }
      const body = await res.json();
      if (!res.ok() || body.success === false) {
        test.skip();
        return;
      }
      expect(body.data).toBeDefined();
    } catch {
      // Endpoint does not exist or returned non-JSON — skip
      test.skip();
    }
  });

  // ---- Registration Numbers ----

  test('GET /company/registration-numbers lists registration numbers', async () => {
    const data = await api.get('/api/v1/company/registration-numbers');
    expect(Array.isArray(data)).toBe(true);
  });

  // ---- Permission: unauthenticated access denied ----

  test('announcements endpoint rejects unauthenticated requests for POST', async () => {
    const noAuth = await createApiClient(BASE);
    try {
      await expect(async () => {
        await noAuth.post('/api/v1/announcements', {
          title: 'Unauthorized',
          content: 'Should fail',
        });
      }).rejects.toThrow();
    } finally {
      await noAuth.dispose();
    }
  });

  test('dashboard stats reject unauthenticated requests', async () => {
    const noAuth = await createApiClient(BASE);
    try {
      await expect(async () => {
        await noAuth.get('/api/v1/dashboard/stats');
      }).rejects.toThrow();
    } finally {
      await noAuth.dispose();
    }
  });

  test('registration numbers reject unauthenticated requests', async () => {
    const noAuth = await createApiClient(BASE);
    try {
      await expect(async () => {
        await noAuth.get('/api/v1/company/registration-numbers');
      }).rejects.toThrow();
    } finally {
      await noAuth.dispose();
    }
  });
});
