import { test, expect } from '@playwright/test';
import { loadState } from '../../fixtures/state';
import { createApiClient } from '../../fixtures/api-client';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Company / Department / Position API', () => {
  test('list departments', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const departments = await api.get('/api/v1/company/departments');

      expect(Array.isArray(departments)).toBe(true);
      expect(departments.length).toBeGreaterThan(0);

      const dept = departments[0];
      expect(dept.id).toBeTruthy();
      expect(dept.name).toBeTruthy();
    } finally {
      await api.dispose();
    }
  });

  test('create department', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const ts = Date.now();
      const name = `E2E-Dept-${ts}`;

      const dept = await api.post('/api/v1/company/departments', {
        name,
        description: `Test department created at ${ts}`,
      });

      expect(dept).toBeTruthy();
      expect(dept.id).toBeTruthy();
      expect(dept.name).toBe(name);
    } finally {
      await api.dispose();
    }
  });

  test('update department', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const deptId = state.departmentIds[0];
      expect(deptId).toBeTruthy();

      const ts = Date.now();
      const newName = `Updated-Dept-${ts}`;

      const updated = await api.put(`/api/v1/company/departments/${deptId}`, {
        name: newName,
        description: 'Updated by e2e test',
      });

      expect(updated).toBeTruthy();
      expect(updated.name).toBe(newName);
    } finally {
      await api.dispose();
    }
  });

  test('list positions', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const positions = await api.get('/api/v1/company/positions');

      expect(Array.isArray(positions)).toBe(true);
      expect(positions.length).toBeGreaterThan(0);

      const pos = positions[0];
      expect(pos.id).toBeTruthy();
      expect(pos.title).toBeTruthy();
    } finally {
      await api.dispose();
    }
  });

  test('create position with department_id', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const ts = Date.now();
      const deptId = state.departmentIds[0];

      const position = await api.post('/api/v1/company/positions', {
        title: `E2E-Position-${ts}`,
        department_id: deptId,
        description: `Test position created at ${ts}`,
      });

      expect(position).toBeTruthy();
      expect(position.id).toBeTruthy();
      expect(position.title).toBe(`E2E-Position-${ts}`);
      expect(position.department_id).toBe(deptId);
    } finally {
      await api.dispose();
    }
  });

  test('list cost centers', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const costCenters = await api.get('/api/v1/company/cost-centers');

      expect(Array.isArray(costCenters)).toBe(true);
      expect(costCenters.length).toBeGreaterThan(0);

      const cc = costCenters[0];
      expect(cc.id).toBeTruthy();
      expect(cc.code).toBeTruthy();
    } finally {
      await api.dispose();
    }
  });

  test('create cost center', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const ts = Date.now();
      const code = `CC-E2E-${ts}`;

      const cc = await api.post('/api/v1/company/cost-centers', {
        code,
        name: `E2E Cost Center ${ts}`,
        description: `Test cost center created at ${ts}`,
      });

      expect(cc).toBeTruthy();
      expect(cc.id).toBeTruthy();
      expect(cc.code).toBe(code);
    } finally {
      await api.dispose();
    }
  });
});
