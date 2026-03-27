import { ApiClient } from '../fixtures/api-client';
import { updateState } from '../fixtures/state';

const LEAVE_TYPES = [
  { code: 'VL', name: 'Vacation Leave', default_days: '15', is_paid: true },
  { code: 'SL', name: 'Sick Leave', default_days: '15', is_paid: true },
  { code: 'ML', name: 'Maternity Leave', default_days: '105', is_paid: true },
  { code: 'PL', name: 'Paternity Leave', default_days: '7', is_paid: true },
  { code: 'SPL', name: 'Solo Parent Leave', default_days: '7', is_paid: true },
  { code: 'EL', name: 'Emergency Leave', default_days: '5', is_paid: false },
];

export async function seedLeaveTypes(api: ApiClient): Promise<void> {
  const leaveTypeIds: number[] = [];

  for (const lt of LEAVE_TYPES) {
    try {
      const res = await api.post('/api/v1/leaves/types', lt);
      leaveTypeIds.push(res.id);
    } catch (err) {
      // 409 = code already exists from previous run; fetch existing
      try {
        const existing = await api.get('/api/v1/leaves/types');
        const match = existing.find((e: any) => e.code === lt.code);
        if (match) leaveTypeIds.push(match.id);
      } catch { /* ignore */ }
    }
  }

  updateState({ leaveTypeIds });
  console.log(`  Leave types: ${leaveTypeIds.length}`);
}
