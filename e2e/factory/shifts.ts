import { ApiClient } from '../fixtures/api-client';
import { updateState } from '../fixtures/state';

const SHIFTS = [
  { name: 'Day Shift', start_time: '08:00', end_time: '17:00' },
  { name: 'Night Shift', start_time: '22:00', end_time: '06:00' },
  { name: 'Mid Shift', start_time: '14:00', end_time: '22:00' },
];

export async function seedShifts(api: ApiClient): Promise<void> {
  const shiftIds: number[] = [];

  for (const shift of SHIFTS) {
    try {
      const res = await api.post('/api/v1/attendance/shifts', shift);
      shiftIds.push(res.id);
    } catch (err) {
      console.warn(`  Warning: Shift '${shift.name}' failed: ${err}`);
    }
  }

  updateState({ shiftIds });
  console.log(`  Shifts: ${shiftIds.length}`);
}
