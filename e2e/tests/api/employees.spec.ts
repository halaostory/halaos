import { test, expect } from '@playwright/test';
import { loadState } from '../../fixtures/state';
import { createApiClient } from '../../fixtures/api-client';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Employees API', () => {
  test('list employees returns array', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const employees = await api.get('/api/v1/employees');

      expect(Array.isArray(employees)).toBe(true);
      expect(employees.length).toBeGreaterThan(0);

      const emp = employees[0];
      expect(emp.id).toBeTruthy();
      expect(emp.first_name).toBeTruthy();
      expect(emp.last_name).toBeTruthy();
    } finally {
      await api.dispose();
    }
  });

  test('list employees with department filter', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const deptId = state.departmentIds[0];
      expect(deptId).toBeTruthy();

      const employees = await api.get('/api/v1/employees', {
        department_id: String(deptId),
      });

      expect(Array.isArray(employees)).toBe(true);
      // All returned employees should belong to the filtered department
      for (const emp of employees) {
        expect(emp.department_id).toBe(deptId);
      }
    } finally {
      await api.dispose();
    }
  });

  test('list employees with pagination', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const page1 = await api.get('/api/v1/employees', {
        limit: '5',
        page: '1',
      });

      expect(Array.isArray(page1)).toBe(true);
      expect(page1.length).toBeLessThanOrEqual(5);

      const page2 = await api.get('/api/v1/employees', {
        limit: '5',
        page: '2',
      });

      expect(Array.isArray(page2)).toBe(true);

      // Pages should have different data (if enough employees exist)
      if (page1.length > 0 && page2.length > 0) {
        expect(page1[0].id).not.toBe(page2[0].id);
      }
    } finally {
      await api.dispose();
    }
  });

  test('get employee by ID', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const empId = state.employeeIds[0];
      expect(empId).toBeTruthy();

      const emp = await api.get(`/api/v1/employees/${empId}`);

      expect(emp).toBeTruthy();
      expect(emp.id).toBe(empId);
      expect(emp.first_name).toBeTruthy();
      expect(emp.last_name).toBeTruthy();
      expect(emp.employee_no).toBeTruthy();
    } finally {
      await api.dispose();
    }
  });

  test('get nonexistent employee returns error', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      const fakeId = 999999999;

      await expect(
        api.get(`/api/v1/employees/${fakeId}`)
      ).rejects.toThrow(/API error/);
    } finally {
      await api.dispose();
    }
  });

  test('create and update employee', async () => {
    const state = loadState();
    const api = await createApiClient(BASE, state.adminToken);

    try {
      // employee_no is VARCHAR(20); keep prefix short to fit timestamp
      const ts = Date.now().toString().slice(-10);
      const empNo = `E2E-N-${ts}`;

      // Create employee
      const created = await api.post('/api/v1/employees', {
        employee_no: empNo,
        first_name: 'TestCreate',
        last_name: 'Employee',
        email: `${empNo.toLowerCase()}@test.halaos.com`,
        hire_date: '2025-01-15',
        employment_type: 'regular',
        department_id: state.departmentIds[0],
        position_id: state.positionIds[0],
        gender: 'male',
        birth_date: '1995-06-15',
      });

      expect(created).toBeTruthy();
      expect(created.id).toBeTruthy();
      expect(created.employee_no).toBe(empNo);
      expect(created.first_name).toBe('TestCreate');

      // Update employee
      const updated = await api.put(`/api/v1/employees/${created.id}`, {
        first_name: 'UpdatedName',
        last_name: 'UpdatedLast',
      });

      expect(updated).toBeTruthy();
      expect(updated.first_name).toBe('UpdatedName');
      expect(updated.last_name).toBe('UpdatedLast');

      // Verify update persisted
      const fetched = await api.get(`/api/v1/employees/${created.id}`);
      expect(fetched.first_name).toBe('UpdatedName');
      expect(fetched.last_name).toBe('UpdatedLast');
    } finally {
      await api.dispose();
    }
  });

  test('employee role cannot create employee', async () => {
    const state = loadState();
    const tokens = Object.values(state.employeeTokens);
    const empToken = tokens[0];
    expect(empToken).toBeTruthy();

    const api = await createApiClient(BASE, empToken);

    try {
      const ts = Date.now();

      await expect(
        api.post('/api/v1/employees', {
          employee_no: `E2E-UNAUTH-${ts}`,
          first_name: 'Unauthorized',
          last_name: 'Attempt',
          hire_date: '2025-01-01',
        })
      ).rejects.toThrow(/API error/);
    } finally {
      await api.dispose();
    }
  });
});
