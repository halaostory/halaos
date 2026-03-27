import { test, expect } from '@playwright/test';
import { loadState } from '../../fixtures/state';
import { createApiClient } from '../../fixtures/api-client';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('201 File / Document Management API', () => {
  let api: Awaited<ReturnType<typeof createApiClient>>;
  let employeeId: number;
  let createdCategoryId: number | null = null;
  let createdRequirementId: number | null = null;

  test.beforeAll(async () => {
    const state = loadState();
    api = await createApiClient(BASE, state.adminToken);
    employeeId = state.employeeIds[0];
  });

  test.afterAll(async () => {
    // Clean up created requirement
    if (createdRequirementId) {
      try {
        await api.delete(`/api/v1/201file/requirements/${createdRequirementId}`);
      } catch {
        // ignore cleanup errors
      }
    }
    await api.dispose();
  });

  // ---- Document Categories ----

  test('GET /201file/categories lists document categories', async () => {
    const data = await api.get('/api/v1/201file/categories');
    expect(Array.isArray(data)).toBe(true);
    expect(data.length).toBeGreaterThan(0);
    // Each category should have name and slug
    const first = data[0];
    expect(first).toHaveProperty('name');
    expect(first).toHaveProperty('slug');
  });

  test('POST /201file/categories creates a new category', async () => {
    const slug = `e2e-test-${Date.now()}`;
    const data = await api.post('/api/v1/201file/categories', {
      name: 'E2E Test Category',
      slug,
      description: 'Created by E2E test',
      sort_order: 99,
    });
    expect(data).toHaveProperty('id');
    expect(data.name).toBe('E2E Test Category');
    expect(data.slug).toBe(slug);
    createdCategoryId = data.id;
  });

  test('POST /201file/categories rejects missing required fields', async () => {
    await expect(async () => {
      await api.post('/api/v1/201file/categories', {
        description: 'no name or slug',
      });
    }).rejects.toThrow();
  });

  // ---- Employee Documents ----

  test('GET /201file/employee/:id lists employee documents', async () => {
    const data = await api.get(`/api/v1/201file/employee/${employeeId}`);
    expect(Array.isArray(data)).toBe(true);
  });

  test('GET /201file/employee/:id with category_id filter', async () => {
    const categories = await api.get('/api/v1/201file/categories');
    if (categories.length > 0) {
      const catId = categories[0].id;
      const data = await api.get(`/api/v1/201file/employee/${employeeId}`, {
        category_id: String(catId),
      });
      expect(Array.isArray(data)).toBe(true);
    }
  });

  // ---- Employee Document Stats ----

  test('GET /201file/employee/:id/stats returns document statistics', async () => {
    const data = await api.get(`/api/v1/201file/employee/${employeeId}/stats`);
    expect(data).toBeDefined();
    expect(data).toHaveProperty('total_documents');
    expect(data).toHaveProperty('expiring_soon');
  });

  // ---- Expiring Documents ----

  test('GET /201file/expiring lists expiring documents', async () => {
    const data = await api.get('/api/v1/201file/expiring');
    expect(Array.isArray(data)).toBe(true);
  });

  // ---- Document Requirements ----

  test('GET /201file/requirements lists document requirements', async () => {
    const data = await api.get('/api/v1/201file/requirements');
    expect(Array.isArray(data)).toBe(true);
  });

  test('POST /201file/requirements creates a document requirement', async () => {
    // We need a category_id; fetch existing categories
    const categories = await api.get('/api/v1/201file/categories');
    expect(categories.length).toBeGreaterThan(0);
    const categoryId = categories[0].id;

    const data = await api.post('/api/v1/201file/requirements', {
      category_id: categoryId,
      document_name: `E2E Test Requirement ${Date.now()}`,
      is_required: true,
      applies_to: 'all',
      expiry_months: 12,
    });
    expect(data).toHaveProperty('id');
    expect(data.document_name).toContain('E2E Test Requirement');
    createdRequirementId = data.id;
  });

  test('POST /201file/requirements rejects missing required fields', async () => {
    await expect(async () => {
      await api.post('/api/v1/201file/requirements', {
        is_required: false,
      });
    }).rejects.toThrow();
  });

  // ---- Permission: unauthenticated access denied ----

  test('document endpoints reject unauthenticated requests', async () => {
    const noAuth = await createApiClient(BASE);
    try {
      await expect(async () => {
        await noAuth.get('/api/v1/201file/categories');
      }).rejects.toThrow();
    } finally {
      await noAuth.dispose();
    }
  });

  // ---- Error: invalid employee ID ----

  test('GET /201file/employee/invalid returns error', async () => {
    await expect(async () => {
      await api.get('/api/v1/201file/employee/not-a-number');
    }).rejects.toThrow();
  });

  test('GET /201file/employee/0/stats returns error or empty', async () => {
    try {
      const data = await api.get('/api/v1/201file/employee/0/stats');
      // If it doesn't throw, it should still return valid structure
      expect(data).toBeDefined();
    } catch {
      // Expected — invalid employee ID
    }
  });
});
