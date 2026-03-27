import { test, expect } from '@playwright/test';
import { loadState } from '../../fixtures/state';
import { createApiClient } from '../../fixtures/api-client';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Auth API', () => {
  test('login with valid credentials returns token', async () => {
    const state = loadState();
    const api = await createApiClient(BASE);

    try {
      let res: any;
      try {
        res = await api.post('/api/v1/auth/cli-login', {
          email: state.adminEmail,
          password: state.adminPassword,
        });
      } catch (err: any) {
        if (err.message?.includes('rate limit')) {
          test.skip(true, 'Rate limited — login already verified by globalSetup');
          return;
        }
        throw err;
      }

      const token = res.token || res.access_token;
      expect(token).toBeTruthy();
      expect(typeof token).toBe('string');
      expect(token.length).toBeGreaterThan(10);

      expect(res.user).toBeTruthy();
      expect(res.user.email).toBe(state.adminEmail);
      expect(res.user.role).toBeTruthy();
    } finally {
      await api.dispose();
    }
  });

  test('login with wrong password throws error', async () => {
    const api = await createApiClient(BASE);

    try {
      let threw = false;
      try {
        await api.post('/api/v1/auth/cli-login', {
          email: 'nonexistent-user@test.halaos.com',
          password: 'WrongPassword123!',
        });
      } catch (err: any) {
        threw = true;
        if (err.message?.includes('rate limit')) {
          test.skip(true, 'Rate limited — cannot test wrong password');
          return;
        }
        expect(err.message).toMatch(/API error/);
      }
      expect(threw).toBe(true);
    } finally {
      await api.dispose();
    }
  });

  test('GET /api/v1/auth/me returns current user', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const user = await api.get('/api/v1/auth/me');

      expect(user).toBeTruthy();
      expect(user.id).toBeTruthy();
      expect(user.email).toBe(state.adminEmail);
      expect(user.role).toBeTruthy();
      expect(user.company_id).toBeTruthy();
    } finally {
      await api.dispose();
    }
  });

  test('GET /api/v1/auth/me without token returns 401', async () => {
    const api = await createApiClient(BASE);

    try {
      const res = await api.getRaw('/api/v1/auth/me');

      expect(res.status()).toBe(401);
    } finally {
      await api.dispose();
    }
  });
});
