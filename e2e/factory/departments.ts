import { ApiClient } from '../fixtures/api-client';
import { updateState } from '../fixtures/state';

const DEPARTMENTS = [
  'Human Resources', 'Engineering', 'Finance', 'Operations',
  'Sales', 'Marketing', 'Legal', 'Customer Support',
];

const POSITIONS = [
  'Manager', 'Senior Developer', 'Junior Developer', 'Designer',
  'Accountant', 'HR Specialist', 'Sales Rep', 'Support Agent',
  'Team Lead', 'Director', 'Analyst', 'Coordinator',
  'Admin Assistant', 'QA Engineer', 'DevOps Engineer',
];

const COST_CENTERS = ['CC-HQ', 'CC-BRANCH-1', 'CC-REMOTE'];

export async function seedDepartments(api: ApiClient): Promise<void> {
  const departmentIds: number[] = [];
  for (const name of DEPARTMENTS) {
    const res = await api.post('/api/v1/company/departments', { name, description: `${name} department` });
    departmentIds.push(res.id);
  }

  const positionIds: number[] = [];
  for (const title of POSITIONS) {
    const deptId = departmentIds[Math.floor(Math.random() * departmentIds.length)];
    const res = await api.post('/api/v1/company/positions', {
      title,
      department_id: deptId,
      description: `${title} role`,
    });
    positionIds.push(res.id);
  }

  const costCenterIds: number[] = [];
  for (const code of COST_CENTERS) {
    const res = await api.post('/api/v1/company/cost-centers', { code, name: code, description: code });
    costCenterIds.push(res.id);
  }

  updateState({ departmentIds, positionIds, costCenterIds });
  console.log(`  Departments: ${departmentIds.length}, Positions: ${positionIds.length}, Cost Centers: ${costCenterIds.length}`);
}
