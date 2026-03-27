import { ApiClient } from '../fixtures/api-client';
import { updateState } from '../fixtures/state';

export async function seedPayroll(api: ApiClient): Promise<void> {
  // Create a payroll cycle
  const now = new Date();
  const startDate = new Date(now.getFullYear(), now.getMonth(), 1).toISOString().split('T')[0];
  const endDate = new Date(now.getFullYear(), now.getMonth(), 15).toISOString().split('T')[0];

  try {
    const cycle = await api.post('/api/v1/payroll/cycles', {
      name: `E2E Payroll ${startDate}`,
      period_start: startDate,
      period_end: endDate,
      pay_date: endDate,
      frequency: 'semi_monthly',
    });
    updateState({ payrollCycleId: cycle.id });
    console.log(`  Payroll cycle: ${cycle.id}`);
  } catch (err) {
    console.warn(`  Warning: Payroll cycle creation failed: ${err}`);
  }
}
