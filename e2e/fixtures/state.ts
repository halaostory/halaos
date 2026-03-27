import { readFileSync, writeFileSync, existsSync } from 'fs';
import { resolve } from 'path';

const STATE_PATH = resolve(__dirname, '../.test-state.json');

export interface TestState {
  companyName: string;
  adminToken: string;
  adminRefreshToken: string;
  adminEmail: string;
  adminPassword: string;
  employeeTokens: Record<string, string>;
  employeeRefreshTokens: Record<string, string>;
  employeeEmails: Record<string, string>;
  employeePasswords: Record<string, string>;
  departmentIds: number[];
  positionIds: number[];
  costCenterIds: number[];
  employeeIds: number[];
  employeeNos: string[];
  leaveTypeIds: number[];
  payrollCycleId: number | null;
  benefitPlanIds: number[];
  trainingProgramIds: number[];
  shiftIds: number[];
  [key: string]: unknown;
}

const DEFAULT_STATE: TestState = {
  companyName: '',
  adminToken: '',
  adminRefreshToken: '',
  adminEmail: '',
  adminPassword: '',
  employeeTokens: {},
  employeeRefreshTokens: {},
  employeeEmails: {},
  employeePasswords: {},
  departmentIds: [],
  positionIds: [],
  costCenterIds: [],
  employeeIds: [],
  employeeNos: [],
  leaveTypeIds: [],
  payrollCycleId: null,
  benefitPlanIds: [],
  trainingProgramIds: [],
  shiftIds: [],
};

let cached: TestState | null = null;

export function loadState(): TestState {
  if (cached) return cached;
  if (!existsSync(STATE_PATH)) return { ...DEFAULT_STATE };
  cached = JSON.parse(readFileSync(STATE_PATH, 'utf-8'));
  return cached!;
}

export function saveState(state: TestState): void {
  cached = state;
  writeFileSync(STATE_PATH, JSON.stringify(state, null, 2));
}

export function updateState(patch: Partial<TestState>): void {
  const state = loadState();
  Object.assign(state, patch);
  saveState(state);
}
