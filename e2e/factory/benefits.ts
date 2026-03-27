import { ApiClient } from '../fixtures/api-client';
import { updateState } from '../fixtures/state';

const BENEFIT_PLANS = [
  { name: 'Health Insurance', type: 'health', description: 'Company health insurance plan' },
  { name: 'Dental Plan', type: 'dental', description: 'Dental coverage plan' },
  { name: 'Life Insurance', type: 'life', description: 'Life insurance benefit' },
];

export async function seedBenefits(api: ApiClient): Promise<void> {
  const benefitPlanIds: number[] = [];

  for (const plan of BENEFIT_PLANS) {
    try {
      const res = await api.post('/api/v1/benefits/plans', plan);
      benefitPlanIds.push(res.id);
    } catch (err) {
      console.warn(`  Warning: Benefit plan '${plan.name}' failed: ${err}`);
    }
  }

  updateState({ benefitPlanIds });
  console.log(`  Benefit plans: ${benefitPlanIds.length}`);
}
