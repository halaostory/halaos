import { createApiClient } from './api-client';
import { loadState } from './state';
import { seedCompany } from '../factory/company';
import { seedDepartments } from '../factory/departments';
import { seedEmployees } from '../factory/employees';
import { seedLeaveTypes } from '../factory/leave';
import { seedAttendance } from '../factory/attendance';
import { seedPayroll } from '../factory/payroll';
import { seedBenefits } from '../factory/benefits';
import { seedTraining } from '../factory/training';
import { seedShifts } from '../factory/shifts';
import { seedAnnouncements } from '../factory/announcements';
import { seedDocuments } from '../factory/documents';

/** Check if JWT has enough remaining validity for the test run (~30 min) */
function isTokenFresh(token: string, marginMinutes = 30): boolean {
  try {
    const payload = token.split('.')[1];
    const padded = payload + '='.repeat(4 - (payload.length % 4));
    const claims = JSON.parse(Buffer.from(padded, 'base64').toString());
    const margin = marginMinutes * 60 * 1000;
    return Date.now() < claims.exp * 1000 - margin;
  } catch {
    return false;
  }
}

/** Check if existing test state has valid data and non-expired JWT */
function isStateValid(): boolean {
  try {
    const state = loadState();
    if (!state.adminToken || !state.adminEmail || !state.adminPassword) return false;
    if (state.employeeIds.length === 0) return false;
    if (!isTokenFresh(state.adminToken)) return false;
    return true;
  } catch {
    return false;
  }
}

async function globalSetup(): Promise<void> {
  const baseURL = process.env.E2E_BASE_URL || 'https://halaos.com';
  console.log(`\n[Data Factory] Starting — target: ${baseURL}`);

  // Reuse existing state if JWT is still valid (set E2E_FORCE_SEED=1 to override)
  if (!process.env.E2E_FORCE_SEED && isStateValid()) {
    const state = loadState();
    console.log('[Data Factory] Reusing existing test data (token still valid)');
    console.log(`  Company: ${state.companyName}`);
    console.log(`  Admin: ${state.adminEmail}`);
    console.log(`  Employees: ${state.employeeIds.length}`);
    return;
  }

  console.log('[Data Factory] Full data seed required');
  const api = await createApiClient(baseURL);
  api.setRetryConfig(5, 310_000);

  try {
    // Phase 1: Dependency modules (abort on failure)
    console.log('[Phase 1] Creating company...');
    await seedCompany(api);

    console.log('[Phase 1] Creating departments, positions, cost centers...');
    await seedDepartments(api);

    console.log('[Phase 1] Creating employees + user accounts...');
    await seedEmployees(api);

    // Phase 2: Leaf modules (log and continue on failure)
    const leafModules: Array<{ name: string; fn: (api: any) => Promise<void> }> = [
      { name: 'Leave Types', fn: seedLeaveTypes },
      { name: 'Shifts', fn: seedShifts },
      { name: 'Attendance', fn: seedAttendance },
      { name: 'Payroll', fn: seedPayroll },
      { name: 'Benefits', fn: seedBenefits },
      { name: 'Training', fn: seedTraining },
      { name: 'Announcements', fn: seedAnnouncements },
      { name: 'Documents', fn: seedDocuments },
    ];

    for (const mod of leafModules) {
      try {
        console.log(`[Phase 2] Seeding ${mod.name}...`);
        await mod.fn(api);
      } catch (err) {
        console.warn(`  [WARN] ${mod.name} seeding failed: ${err}`);
      }
    }

    const state = loadState();
    console.log(`\n[Data Factory] Complete!`);
    console.log(`  Company: ${state.companyName}`);
    console.log(`  Departments: ${state.departmentIds.length}`);
    console.log(`  Employees: ${state.employeeIds.length}`);
    console.log(`  Employee tokens: ${Object.keys(state.employeeTokens).length}`);
  } finally {
    await api.dispose();
  }
}

export default globalSetup;
