import { ApiClient } from '../fixtures/api-client';
import { updateState } from '../fixtures/state';

const LEAVE_TYPES = [
  { name: 'Vacation Leave', days_per_year: 15, is_paid: true },
  { name: 'Sick Leave', days_per_year: 15, is_paid: true },
  { name: 'Maternity Leave', days_per_year: 105, is_paid: true },
  { name: 'Paternity Leave', days_per_year: 7, is_paid: true },
  { name: 'Solo Parent Leave', days_per_year: 7, is_paid: true },
  { name: 'Emergency Leave', days_per_year: 5, is_paid: false },
];

export async function seedLeaveTypes(api: ApiClient): Promise<void> {
  const leaveTypeIds: number[] = [];

  for (const lt of LEAVE_TYPES) {
    const res = await api.post('/api/v1/leaves/types', lt);
    leaveTypeIds.push(res.id);
  }

  updateState({ leaveTypeIds });
  console.log(`  Leave types: ${leaveTypeIds.length}`);
}
