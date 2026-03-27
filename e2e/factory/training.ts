import { ApiClient } from '../fixtures/api-client';
import { updateState } from '../fixtures/state';

const PROGRAMS = [
  { title: 'Onboarding Training', description: 'New hire orientation', training_type: 'onboarding' },
  { title: 'Safety Training', description: 'Workplace safety procedures', training_type: 'compliance' },
  { title: 'Leadership Workshop', description: 'Management skills development', training_type: 'development' },
];

export async function seedTraining(api: ApiClient): Promise<void> {
  const trainingProgramIds: number[] = [];
  const startDate = new Date();
  const endDate = new Date(startDate);
  endDate.setMonth(endDate.getMonth() + 1);

  for (const prog of PROGRAMS) {
    try {
      const res = await api.post('/api/v1/trainings', {
        ...prog,
        start_date: startDate.toISOString().split('T')[0],
        end_date: endDate.toISOString().split('T')[0],
        max_participants: 30,
      });
      trainingProgramIds.push(res.id);
    } catch (err) {
      console.warn(`  Warning: Training '${prog.title}' failed: ${err}`);
    }
  }

  updateState({ trainingProgramIds });
  console.log(`  Training programs: ${trainingProgramIds.length}`);
}
