import { ApiClient, createApiClient } from '../fixtures/api-client';
import { loadState, updateState } from '../fixtures/state';

const FIRST_NAMES = [
  'Juan', 'Maria', 'Jose', 'Ana', 'Pedro', 'Rosa', 'Carlos', 'Elena', 'Miguel', 'Carmen',
  'Antonio', 'Lucia', 'Rafael', 'Isabel', 'Luis', 'Teresa', 'Manuel', 'Gloria', 'Francisco', 'Patricia',
];
const LAST_NAMES = [
  'Santos', 'Reyes', 'Cruz', 'Garcia', 'Torres', 'Lopez', 'Ramos', 'Flores', 'Rivera', 'Gomez',
  'Diaz', 'Mendoza', 'Morales', 'Castro', 'Ortiz', 'Vargas', 'Romero', 'Herrera', 'Medina', 'Aguilar',
];
const EMPLOYMENT_TYPES = ['regular', 'contractual', 'probationary'];

export async function seedEmployees(api: ApiClient): Promise<void> {
  const state = loadState();
  const employeeIds: number[] = [];
  const employeeNos: string[] = [];

  // Use timestamp suffix to make emails unique per seed run (avoids stale user accounts from previous runs)
  const ts = Date.now();
  const emailSuffix = `${ts}`;

  for (let i = 0; i < 20; i++) {
    const empNo = `E2E-${String(i + 1).padStart(3, '0')}`;
    const firstName = FIRST_NAMES[i % FIRST_NAMES.length];
    const lastName = LAST_NAMES[i % LAST_NAMES.length];
    const deptId = state.departmentIds[i % state.departmentIds.length];
    const posId = state.positionIds[i % state.positionIds.length];
    const hireDate = new Date(2024, 0, 1 + i).toISOString().split('T')[0];

    const res = await api.post('/api/v1/employees', {
      employee_no: empNo,
      first_name: firstName,
      last_name: lastName,
      email: `${empNo.toLowerCase()}-${emailSuffix}@test.halaos.com`,
      hire_date: hireDate,
      employment_type: EMPLOYMENT_TYPES[i % EMPLOYMENT_TYPES.length],
      department_id: deptId,
      position_id: posId,
      gender: i % 2 === 0 ? 'male' : 'female',
      birth_date: new Date(1990 + (i % 15), i % 12, 1 + (i % 28)).toISOString().split('T')[0],
    });
    employeeIds.push(res.id);
    employeeNos.push(empNo);
  }

  // Create user accounts for first 10 employees and get tokens
  const employeeTokens: Record<string, string> = {};
  const employeeRefreshTokens: Record<string, string> = {};
  const employeeEmails: Record<string, string> = {};
  const employeePasswords: Record<string, string> = {};
  const baseURL = process.env.E2E_BASE_URL || 'https://halaos.com';

  for (let i = 0; i < 3; i++) {
    const empNo = employeeNos[i];
    const email = `${empNo.toLowerCase()}-${emailSuffix}@test.halaos.com`;
    const password = `EmpPass${i + 1}!abc`;

    try {
      // Create user account linked to employee (admin endpoint)
      await api.post('/api/v1/users/employee-account', {
        employee_id: employeeIds[i],
        email,
        password,
        role: i === 0 ? 'manager' : 'employee',
      });

      // Login to get token (use higher retry config like globalSetup)
      const loginApi = await createApiClient(baseURL);
      loginApi.setRetryConfig(5, 310_000);
      const loginRes = await loginApi.post('/api/v1/auth/cli-login', { email, password });
      employeeTokens[empNo] = loginRes.token || loginRes.access_token;
      employeeRefreshTokens[empNo] = loginRes.refresh_token || '';
      employeeEmails[empNo] = email;
      employeePasswords[empNo] = password;
      await loginApi.dispose();
    } catch (err) {
      console.warn(`  Warning: Could not create user for ${empNo}: ${err}`);
    }
  }

  updateState({ employeeIds, employeeNos, employeeTokens, employeeRefreshTokens, employeeEmails, employeePasswords });
  console.log(`  Employees: ${employeeIds.length}, User accounts: ${Object.keys(employeeTokens).length}`);
}
