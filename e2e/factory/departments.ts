import { ApiClient } from '../fixtures/api-client';
import { updateState } from '../fixtures/state';

const DEPARTMENTS = [
  { code: 'HR', name: 'Human Resources' },
  { code: 'ENG', name: 'Engineering' },
  { code: 'FIN', name: 'Finance' },
  { code: 'OPS', name: 'Operations' },
  { code: 'SAL', name: 'Sales' },
  { code: 'MKT', name: 'Marketing' },
  { code: 'LEG', name: 'Legal' },
  { code: 'SUP', name: 'Customer Support' },
];

const POSITIONS = [
  { code: 'MGR', title: 'Manager' },
  { code: 'SR-DEV', title: 'Senior Developer' },
  { code: 'JR-DEV', title: 'Junior Developer' },
  { code: 'DSG', title: 'Designer' },
  { code: 'ACCT', title: 'Accountant' },
  { code: 'HR-SP', title: 'HR Specialist' },
  { code: 'SALES', title: 'Sales Rep' },
  { code: 'SUP-AG', title: 'Support Agent' },
  { code: 'TL', title: 'Team Lead' },
  { code: 'DIR', title: 'Director' },
  { code: 'ANL', title: 'Analyst' },
  { code: 'COORD', title: 'Coordinator' },
  { code: 'ADMIN', title: 'Admin Assistant' },
  { code: 'QA', title: 'QA Engineer' },
  { code: 'DEVOPS', title: 'DevOps Engineer' },
];

const COST_CENTERS = ['CC-HQ', 'CC-BRANCH-1', 'CC-REMOTE'];

export async function seedDepartments(api: ApiClient): Promise<void> {
  const departmentIds: number[] = [];
  for (const dept of DEPARTMENTS) {
    const res = await api.post('/api/v1/company/departments', { code: dept.code, name: dept.name });
    departmentIds.push(res.id);
  }

  const positionIds: number[] = [];
  for (const pos of POSITIONS) {
    const deptId = departmentIds[Math.floor(Math.random() * departmentIds.length)];
    const res = await api.post('/api/v1/company/positions', {
      code: pos.code,
      title: pos.title,
      department_id: deptId,
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
