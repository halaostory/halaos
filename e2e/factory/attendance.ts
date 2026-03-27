import { ApiClient, createApiClient } from '../fixtures/api-client';
import { loadState } from '../fixtures/state';

export async function seedAttendance(api: ApiClient): Promise<void> {
  const state = loadState();
  const baseURL = process.env.E2E_BASE_URL || 'https://halaos.com';

  // Clock in/out for first 5 employees who have tokens
  const empNos = Object.keys(state.employeeTokens).slice(0, 5);
  let clockedIn = 0;

  for (const empNo of empNos) {
    const token = state.employeeTokens[empNo];
    const empApi = await createApiClient(baseURL, token);

    try {
      // Clock in
      await empApi.post('/api/v1/attendance/clock-in', {});
      clockedIn++;

      // Clock out after a short delay
      await new Promise(r => setTimeout(r, 1000));
      await empApi.post('/api/v1/attendance/clock-out', {});
    } catch (err) {
      console.warn(`  Warning: Attendance for ${empNo}: ${err}`);
    } finally {
      await empApi.dispose();
    }
  }

  console.log(`  Attendance records: ${clockedIn} employees clocked in/out`);
}
