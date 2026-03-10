import { ofetch } from "ofetch";

let isRefreshing = false;
let refreshPromise: Promise<boolean> | null = null;

async function tryRefreshToken(): Promise<boolean> {
  const refreshToken = localStorage.getItem("refresh_token");
  if (!refreshToken) return false;

  try {
    const res = await ofetch<{
      data?: { token: string; refresh_token: string };
    }>((import.meta.env.VITE_API_URL || "/api") + "/v1/auth/refresh", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: { refresh_token: refreshToken },
    });
    const data =
      res.data || (res as unknown as { token: string; refresh_token: string });
    if (data.token) {
      localStorage.setItem("token", data.token);
      if (data.refresh_token)
        localStorage.setItem("refresh_token", data.refresh_token);
      return true;
    }
    return false;
  } catch {
    return false;
  }
}

const api = ofetch.create({
  baseURL: import.meta.env.VITE_API_URL || "/api",
  headers: { "Content-Type": "application/json" },
  onRequest({ options }) {
    const token = localStorage.getItem("token");
    if (token) {
      const headers = new Headers(options.headers as HeadersInit);
      headers.set("Authorization", `Bearer ${token}`);
      options.headers = headers;
    }
  },
  async onResponseError({ response, request, options }) {
    if (response.status === 401) {
      // Skip refresh for auth endpoints to avoid loops
      const url =
        typeof request === "string" ? request : request?.toString() || "";
      if (url.includes("/auth/login")) {
        // Let login 401 errors propagate to the caller for proper error display
        return;
      }
      if (url.includes("/auth/refresh")) {
        localStorage.removeItem("token");
        localStorage.removeItem("refresh_token");
        window.location.href = "/login";
        return;
      }

      // Deduplicate concurrent refresh attempts
      if (!isRefreshing) {
        isRefreshing = true;
        refreshPromise = tryRefreshToken().finally(() => {
          isRefreshing = false;
          refreshPromise = null;
        });
      }

      const refreshed = await refreshPromise;
      if (refreshed) {
        // Retry the original request with new token
        const newToken = localStorage.getItem("token");
        const headers = new Headers(options.headers as HeadersInit);
        headers.set("Authorization", `Bearer ${newToken}`);
        await ofetch(request, { ...options, headers });
        return;
      }

      localStorage.removeItem("token");
      localStorage.removeItem("refresh_token");
      window.location.href = "/login";
    }
  },
});

function get<T>(url: string, params?: Record<string, string>) {
  return api<T>(url, { method: "GET", params });
}
function post<T>(url: string, body?: Record<string, unknown>) {
  return api<T>(url, { method: "POST", body });
}
function put<T>(url: string, body?: Record<string, unknown>) {
  return api<T>(url, { method: "PUT", body });
}
// Auth
export const authAPI = {
  register: (data: {
    company_name: string;
    email: string;
    password: string;
    first_name: string;
    last_name: string;
  }) => post("/v1/auth/register", data),
  login: (data: { email: string; password: string }) =>
    post("/v1/auth/login", data),
  refresh: (refresh_token: string) =>
    post("/v1/auth/refresh", { refresh_token }),
  me: () => get("/v1/auth/me"),
  changePassword: (data: { current_password: string; new_password: string }) =>
    put("/v1/auth/password", data),
  updateProfile: (data: {
    first_name: string;
    last_name: string;
    locale?: string;
  }) => put("/v1/auth/profile", data),
  uploadAvatar: (formData: FormData) =>
    api("/v1/auth/avatar", { method: "POST", body: formData, headers: {} }),
};

// Company
export const companyAPI = {
  get: () => get("/v1/company"),
  update: (data: Record<string, unknown>) => put("/v1/company", data),
  uploadLogo: (formData: FormData) =>
    api("/v1/company/logo", { method: "POST", body: formData, headers: {} }),
  listDepartments: () => get("/v1/company/departments"),
  createDepartment: (data: {
    code: string;
    name: string;
    parent_id?: number;
  }) => post("/v1/company/departments", data),
  updateDepartment: (id: number, data: Record<string, unknown>) =>
    put(`/v1/company/departments/${id}`, data),
  listPositions: () => get("/v1/company/positions"),
  createPosition: (data: {
    code: string;
    title: string;
    department_id?: number;
    grade?: string;
  }) => post("/v1/company/positions", data),
  updatePosition: (id: number, data: Record<string, unknown>) =>
    put(`/v1/company/positions/${id}`, data),
  listCostCenters: () => get("/v1/company/cost-centers"),
  createCostCenter: (data: { code: string; name: string }) =>
    post("/v1/company/cost-centers", data),
};

// Employees
export const employeeAPI = {
  list: (params?: Record<string, string>) => get("/v1/employees", params),
  get: (id: number) => get(`/v1/employees/${id}`),
  create: (data: Record<string, unknown>) => post("/v1/employees", data),
  update: (id: number, data: Record<string, unknown>) =>
    put(`/v1/employees/${id}`, data),
  getProfile: (id: number) => get(`/v1/employees/${id}/profile`),
  updateProfile: (id: number, data: Record<string, unknown>) =>
    put(`/v1/employees/${id}/profile`, data),
  listDocuments: (id: number) => get(`/v1/employees/${id}/documents`),
  uploadDocument: (id: number, formData: FormData) =>
    api(`/v1/employees/${id}/documents`, {
      method: "POST",
      body: formData,
      headers: {},
    }),
  downloadDocumentUrl: (id: number, docId: string) =>
    `${import.meta.env.VITE_API_URL || "/api"}/v1/employees/${id}/documents/${docId}/download`,
  deleteDocument: (id: number, docId: string) =>
    api(`/v1/employees/${id}/documents/${docId}`, { method: "DELETE" }),
  getSalary: (id: number) => get(`/v1/employees/${id}/salary`),
  assignSalary: (id: number, data: Record<string, unknown>) =>
    post(`/v1/employees/${id}/salary`, data),
  downloadCOEUrl: (id: number) =>
    `${import.meta.env.VITE_API_URL || "/api"}/v1/employees/${id}/coe`,
  getTimeline: (id: number) => get(`/v1/employees/${id}/timeline`),
  changeStatus: (id: number, data: { status: string; remarks?: string }) =>
    put(`/v1/employees/${id}/status`, data),
  generateLetter: (id: number, data: Record<string, unknown>) =>
    api(`/v1/employees/${id}/letters`, {
      method: "POST",
      body: JSON.stringify(data),
      responseType: "blob",
    }),
  generateLetterUrl: (id: number) =>
    `${import.meta.env.VITE_API_URL || "/api"}/v1/employees/${id}/letters`,
  listExpiringDocuments: () => get("/v1/employees/documents/expiring"),
  bulkSalaryUpdate: (data: {
    employee_ids: number[];
    update_type: "percentage" | "fixed";
    value: number;
    effective_from: string;
    remarks?: string;
  }) => post("/v1/employees/salary/bulk-update", data),
};

// Attendance
export const attendanceAPI = {
  clockIn: (data: {
    source?: string;
    lat?: string;
    lng?: string;
    note?: string;
  }) => post("/v1/attendance/clock-in", data),
  clockOut: (data: {
    source?: string;
    lat?: string;
    lng?: string;
    note?: string;
  }) => post("/v1/attendance/clock-out", data),
  listRecords: (params?: Record<string, string>) =>
    get("/v1/attendance/records", params),
  getSummary: () => get("/v1/attendance/summary"),
  listShifts: () => get("/v1/attendance/shifts"),
  createShift: (data: Record<string, unknown>) =>
    post("/v1/attendance/shifts", data),
  listSchedules: (params?: Record<string, string>) =>
    get("/v1/attendance/schedules", params),
  assignSchedule: (data: Record<string, unknown>) =>
    post("/v1/attendance/schedules", data),
  bulkAssignSchedule: (data: Record<string, unknown>) =>
    post("/v1/attendance/schedules/bulk", data),
  deleteSchedule: (id: number) =>
    api(`/v1/attendance/schedules/${id}`, { method: "DELETE" }),
  getReport: (start: string, end: string) =>
    get("/v1/attendance/report", { start, end }),
  // Schedule Templates
  listScheduleTemplates: () => get("/v1/attendance/schedule-templates"),
  createScheduleTemplate: (data: Record<string, unknown>) =>
    post("/v1/attendance/schedule-templates", data),
  getScheduleTemplate: (id: number) =>
    get(`/v1/attendance/schedule-templates/${id}`),
  updateScheduleTemplate: (id: number, data: Record<string, unknown>) =>
    put(`/v1/attendance/schedule-templates/${id}`, data),
  deleteScheduleTemplate: (id: number) =>
    api(`/v1/attendance/schedule-templates/${id}`, { method: "DELETE" }),
  assignScheduleTemplate: (id: number, data: Record<string, unknown>) =>
    post(`/v1/attendance/schedule-templates/${id}/assign`, data),
  listScheduleAssignments: () => get("/v1/attendance/schedule-assignments"),
  // Attendance Corrections
  createCorrection: (data: {
    attendance_id?: number;
    correction_date: string;
    requested_clock_in?: string;
    requested_clock_out?: string;
    reason: string;
  }) => post("/v1/attendance/corrections", data),
  listCorrections: () => get("/v1/attendance/corrections"),
  listPendingCorrections: () => get("/v1/attendance/corrections/pending"),
  listMyCorrections: () => get("/v1/attendance/corrections/my"),
  approveCorrection: (id: number, note?: string) =>
    post(`/v1/attendance/corrections/${id}/approve`, { note }),
  rejectCorrection: (id: number, note?: string) =>
    post(`/v1/attendance/corrections/${id}/reject`, { note }),
};

// Leave
export const leaveAPI = {
  listTypes: () => get("/v1/leaves/types"),
  createType: (data: Record<string, unknown>) => post("/v1/leaves/types", data),
  getBalances: () => get("/v1/leaves/balances"),
  listRequests: (params?: Record<string, string>) =>
    get("/v1/leaves/requests", params),
  createRequest: (data: Record<string, unknown>) =>
    post("/v1/leaves/requests", data),
  approveRequest: (id: number) => post(`/v1/leaves/requests/${id}/approve`),
  rejectRequest: (id: number, data: { reason?: string }) =>
    post(`/v1/leaves/requests/${id}/reject`, data),
  cancelRequest: (id: number) => post(`/v1/leaves/requests/${id}/cancel`),
  listAllBalances: (params?: Record<string, string>) =>
    get("/v1/leaves/balances/all", params),
  adjustBalance: (data: {
    employee_id: number;
    leave_type_id: number;
    year: number;
    adjusted: number;
  }) => put("/v1/leaves/balances/adjust", data),
  calendar: (start: string, end: string) =>
    get("/v1/leaves/calendar", { start, end }),
  carryover: (fromYear: number, toYear: number) =>
    post("/v1/leaves/carryover", { from_year: fromYear, to_year: toYear }),
};

// Leave Encashment
export const leaveEncashmentAPI = {
  getConvertible: (params?: Record<string, string>) =>
    get("/v1/leaves/encashment/convertible", params),
  create: (data: Record<string, unknown>) =>
    post("/v1/leaves/encashment", data),
  list: (params?: Record<string, string>) =>
    get("/v1/leaves/encashment", params),
  approve: (id: number) => post(`/v1/leaves/encashment/${id}/approve`),
  reject: (id: number, data?: { remarks?: string }) =>
    post(`/v1/leaves/encashment/${id}/reject`, data),
  markPaid: (id: number) => post(`/v1/leaves/encashment/${id}/paid`),
};

// Overtime
export const overtimeAPI = {
  listRequests: (params?: Record<string, string>) =>
    get("/v1/overtime/requests", params),
  createRequest: (data: Record<string, unknown>) =>
    post("/v1/overtime/requests", data),
  approveRequest: (id: number) => post(`/v1/overtime/requests/${id}/approve`),
  rejectRequest: (id: number, data: { reason?: string }) =>
    post(`/v1/overtime/requests/${id}/reject`, data),
};

// Payroll
export const payrollAPI = {
  listCycles: (params?: Record<string, string>) =>
    get("/v1/payroll/cycles", params),
  createCycle: (data: Record<string, unknown>) =>
    post("/v1/payroll/cycles", data),
  runPayroll: (data: { cycle_id: number; run_type?: string }) =>
    post("/v1/payroll/runs", data),
  listItems: (runId: number) => get(`/v1/payroll/runs/${runId}/items`),
  listCycleItems: (cycleId: number) =>
    get(`/v1/payroll/cycles/${cycleId}/items`),
  getPayslip: (id: string) => get(`/v1/payroll/payslips/${id}`),
  payslipPdfUrl: (id: string) =>
    `${import.meta.env.VITE_API_URL || "/api"}/v1/payroll/payslips/${id}/pdf`,
  approveCycle: (id: number) => post(`/v1/payroll/cycles/${id}/approve`),
  lockCycle: (id: number) => post(`/v1/payroll/cycles/${id}/lock`),
  unlockCycle: (id: number) => post(`/v1/payroll/cycles/${id}/unlock`),
  scanAnomalies: (cycleId: number) =>
    get(`/v1/payroll/cycles/${cycleId}/anomalies`),
  listPayslips: (params?: Record<string, string>) =>
    get("/v1/payroll/payslips", params),
  getAutoConfig: () => get("/v1/payroll/auto-config"),
  updateAutoConfig: (data: Record<string, unknown>) =>
    put("/v1/payroll/auto-config", data),
  listAutoLogs: (params?: Record<string, string>) =>
    get("/v1/payroll/auto-logs", params),
};

// Approvals
export const approvalAPI = {
  listPending: () => get("/v1/approvals/pending"),
  approve: (id: number, data?: { comments?: string }) =>
    post(`/v1/approvals/${id}/approve`, data),
  reject: (id: number, data?: { comments?: string }) =>
    post(`/v1/approvals/${id}/reject`, data),
  getContext: (entityType: string, entityId: number) =>
    get("/v1/approvals/context", {
      entity_type: entityType,
      entity_id: String(entityId),
    }),
};

// Compliance
export const complianceAPI = {
  listSSS: () => get("/v1/compliance/sss-table"),
  listPhilHealth: () => get("/v1/compliance/philhealth-table"),
  listPagIBIG: () => get("/v1/compliance/pagibig-table"),
  listBIRTax: () => get("/v1/compliance/bir-tax-table"),
  listGovernmentForms: () => get("/v1/compliance/government-forms"),
  createGovernmentForm: (data: Record<string, unknown>) =>
    post("/v1/compliance/government-forms", data),
  generateForm: (data: Record<string, unknown>) =>
    post("/v1/compliance/government-forms/generate", data),
};

// Salary
export const salaryAPI = {
  listStructures: () => get("/v1/salary/structures"),
  createStructure: (data: { name: string; description?: string }) =>
    post("/v1/salary/structures", data),
  listComponents: () => get("/v1/salary/components"),
  createComponent: (data: Record<string, unknown>) =>
    post("/v1/salary/components", data),
};

// Dashboard
export const dashboardAPI = {
  getStats: () => get("/v1/dashboard/stats"),
  getAttendance: () => get("/v1/dashboard/attendance"),
  getDepartmentDistribution: () => get("/v1/dashboard/department-distribution"),
  getPayrollTrend: () => get("/v1/dashboard/payroll-trend"),
  getLeaveSummary: () => get("/v1/dashboard/leave-summary"),
  getCelebrations: () => get("/v1/dashboard/celebrations"),
  getActionItems: () => get("/v1/dashboard/action-items"),
  getFlightRisk: () => get("/v1/dashboard/flight-risk"),
};

// Flight Risk (convenience alias)
export const flightRiskAPI = {
  getTopRisks: () => get("/v1/dashboard/flight-risk"),
};

export const teamHealthAPI = {
  getScores: () => get("/v1/dashboard/team-health"),
};

export const burnoutRiskAPI = {
  getTopRisks: () => get("/v1/dashboard/burnout-risk"),
};

export const complianceAlertsAPI = {
  getAlerts: () => get("/v1/dashboard/compliance-alerts"),
};

// Holidays
export const holidayAPI = {
  list: (params?: Record<string, string>) => get("/v1/holidays", params),
  create: (data: Record<string, unknown>) => post("/v1/holidays", data),
  delete: (id: number) => api(`/v1/holidays/${id}`, { method: "DELETE" }),
};

// 13th Month Pay
export const thirteenthMonthAPI = {
  list: (params?: Record<string, string>) =>
    get("/v1/payroll/13th-month", params),
  calculate: (data: { year: number }) =>
    post("/v1/payroll/13th-month/calculate", data),
};

// Onboarding
export const onboardingAPI = {
  listTemplates: () => get("/v1/onboarding/templates"),
  createTemplate: (data: Record<string, unknown>) =>
    post("/v1/onboarding/templates", data),
  initiate: (data: {
    employee_id: number;
    workflow_type: string;
    reference_date?: string;
  }) => post("/v1/onboarding/initiate", data),
  listPendingTasks: (params?: Record<string, string>) =>
    get("/v1/onboarding/tasks/pending", params),
  listTasks: (employeeId: number, params?: Record<string, string>) =>
    get(`/v1/onboarding/employees/${employeeId}/tasks`, params),
  getProgress: (employeeId: number) =>
    get(`/v1/onboarding/employees/${employeeId}/progress`),
  updateTask: (id: number, data: { status: string; notes?: string }) =>
    put(`/v1/onboarding/tasks/${id}`, data),
};

// Loans
export const loanAPI = {
  listTypes: () => get("/v1/loans/types"),
  createType: (data: Record<string, unknown>) => post("/v1/loans/types", data),
  list: () => get("/v1/loans"),
  listMy: () => get("/v1/loans/my"),
  apply: (data: Record<string, unknown>) => post("/v1/loans", data),
  get: (id: number) => get(`/v1/loans/${id}`),
  approve: (id: number) => post(`/v1/loans/${id}/approve`),
  cancel: (id: number) => post(`/v1/loans/${id}/cancel`),
  recordPayment: (id: number, data: Record<string, unknown>) =>
    post(`/v1/loans/${id}/payments`, data),
};

// Analytics
export const analyticsAPI = {
  getSummary: () => get("/v1/analytics/summary"),
  getHeadcountTrend: (params?: Record<string, string>) =>
    get("/v1/analytics/headcount-trend", params),
  getTurnover: (params?: Record<string, string>) =>
    get("/v1/analytics/turnover", params),
  getDepartmentCosts: () => get("/v1/analytics/department-costs"),
  getAttendancePatterns: (params?: Record<string, string>) =>
    get("/v1/analytics/attendance-patterns", params),
  getEmploymentTypes: () => get("/v1/analytics/employment-types"),
  getLeaveUtilization: (params?: Record<string, string>) =>
    get("/v1/analytics/leave-utilization", params),
  getBlindSpots: () => get("/v1/analytics/blind-spots"),
  exportCSV: async () => {
    const baseURL = import.meta.env.VITE_API_URL || "/api";
    const token = localStorage.getItem("token");
    const resp = await fetch(`${baseURL}/v1/analytics/export/csv`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    const blob = await resp.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `analytics_${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  },
};

// Org Intelligence
export const orgIntelligenceAPI = {
  getOverview: () => get("/v1/org-intelligence/overview"),
  getTrends: (params?: Record<string, string>) =>
    get("/v1/org-intelligence/trends", params),
  getRiskDistribution: () => get("/v1/org-intelligence/risk-distribution"),
  getCorrelations: () => get("/v1/org-intelligence/correlations"),
  getEmployeeTrends: (id: number, params?: Record<string, string>) =>
    get(`/v1/org-intelligence/employee/${id}/trends`, params),
  getDepartmentTrends: (id: number, params?: Record<string, string>) =>
    get(`/v1/org-intelligence/department/${id}/trends`, params),
  getExecutiveBriefing: () => get("/v1/org-intelligence/executive-briefing"),
  generateBriefing: () =>
    post("/v1/org-intelligence/executive-briefing/generate"),
};

// Self-Service
export const selfServiceAPI = {
  getMyInfo: () => get("/v1/self-service/info"),
  getMyTeam: () => get("/v1/self-service/team"),
  getMyCompensation: () => get("/v1/self-service/compensation"),
  getMyOnboarding: () => get("/v1/self-service/onboarding"),
};

// Performance
export const performanceAPI = {
  listCycles: () => get("/v1/performance/cycles"),
  createCycle: (data: Record<string, unknown>) =>
    post("/v1/performance/cycles", data),
  initiateCycle: (id: number) => post(`/v1/performance/cycles/${id}/initiate`),
  listReviewsByCycle: (id: number) =>
    get(`/v1/performance/cycles/${id}/reviews`),
  listMyReviews: () => get("/v1/performance/reviews/my"),
  getReview: (id: number) => get(`/v1/performance/reviews/${id}`),
  submitSelfReview: (id: number, data: Record<string, unknown>) =>
    put(`/v1/performance/reviews/${id}/self`, data),
  submitManagerReview: (id: number, data: Record<string, unknown>) =>
    put(`/v1/performance/reviews/${id}/manager`, data),
  listGoals: (params?: Record<string, string>) =>
    get("/v1/performance/goals", params),
  createGoal: (data: Record<string, unknown>) =>
    post("/v1/performance/goals", data),
  updateGoal: (id: number, data: Record<string, unknown>) =>
    put(`/v1/performance/goals/${id}`, data),
};

// Export
export const exportAPI = {
  payrollCSV: (cycleId: number) =>
    `${import.meta.env.VITE_API_URL || "/api"}/v1/export/payroll/${cycleId}/csv`,
  employeesCSV: () =>
    `${import.meta.env.VITE_API_URL || "/api"}/v1/export/employees/csv`,
  attendanceCSV: (start: string, end: string) =>
    `${import.meta.env.VITE_API_URL || "/api"}/v1/export/attendance/csv?start=${start}&end=${end}`,
  leaveBalancesCSV: (year?: number) =>
    `${import.meta.env.VITE_API_URL || "/api"}/v1/export/leave-balances/csv${year ? `?year=${year}` : ""}`,
  payrollBankFile: (cycleId: number, format: string = "generic") =>
    `${import.meta.env.VITE_API_URL || "/api"}/v1/export/payroll/${cycleId}/bank-file?format=${format}`,
};

// Import
export const importAPI = {
  employeesCSV: (formData: FormData) =>
    api("/v1/import/employees/csv", {
      method: "POST",
      body: formData,
      headers: {},
    }),
  previewEmployeesCSV: (formData: FormData) =>
    api("/v1/import/employees/preview", {
      method: "POST",
      body: formData,
      headers: {},
    }),
};

// Knowledge Base
export const knowledgeAPI = {
  search: (q: string) => get("/v1/knowledge/search", { q }),
  list: (params?: Record<string, string>) => get("/v1/knowledge", params),
  categories: () => get("/v1/knowledge/categories"),
  get: (id: number) => get(`/v1/knowledge/${id}`),
  create: (data: Record<string, unknown>) => post("/v1/knowledge", data),
  update: (id: number, data: Record<string, unknown>) =>
    put(`/v1/knowledge/${id}`, data),
  delete: (id: number) => api(`/v1/knowledge/${id}`, { method: "DELETE" }),
};

// Notifications
export const notificationAPI = {
  list: () => get("/v1/notifications"),
  unreadCount: () =>
    get<{ data: { count: number } }>("/v1/notifications/unread-count"),
  markRead: (id: number) => post(`/v1/notifications/${id}/read`),
  markAllRead: () => post("/v1/notifications/read-all"),
  delete: (id: number) => api(`/v1/notifications/${id}`, { method: "DELETE" }),
  executeAction: (
    id: number,
    action: string,
    params?: Record<string, unknown>,
  ) => post(`/v1/notifications/${id}/execute-action`, { action, params }),
};

// User Management
export const userAPI = {
  list: (params?: Record<string, string>) => get("/v1/users", params),
  updateRole: (id: number, role: string) =>
    put(`/v1/users/${id}/role`, { role }),
  updateStatus: (id: number, status: string) =>
    put(`/v1/users/${id}/status`, { status }),
  resetPassword: (id: number, password: string) =>
    post(`/v1/users/${id}/reset-password`, { password }),
};

// Announcements
export const announcementAPI = {
  list: () => get("/v1/announcements"),
  listAll: () => get("/v1/announcements/all"),
  create: (data: Record<string, unknown>) => post("/v1/announcements", data),
  delete: (id: number) => api(`/v1/announcements/${id}`, { method: "DELETE" }),
};

// Smart Suggestions
export const suggestionsAPI = {
  list: () => get("/v1/dashboard/suggestions"),
};

// Employee Directory
export const directoryAPI = {
  list: (params?: Record<string, string>) => get("/v1/directory", params),
  orgChart: () => get("/v1/directory/org-chart"),
};

// Audit Trail
export const auditAPI = {
  list: (params?: Record<string, string>) => get("/v1/audit/logs", params),
};

// Reports
export const reportsAPI = {
  doleRegisterUrl: () =>
    `${import.meta.env.VITE_API_URL || "/api"}/v1/reports/dole-register`,
  dtr: (params: Record<string, string>) => get("/v1/reports/dtr", params),
  dtrCsvUrl: (start: string, end: string) =>
    `${import.meta.env.VITE_API_URL || "/api"}/v1/reports/dtr/csv?start=${start}&end=${end}`,
};

// Clearance
export const clearanceAPI = {
  create: (data: Record<string, unknown>) => post("/v1/clearance", data),
  list: (params?: Record<string, string>) => get("/v1/clearance", params),
  get: (id: number) => get(`/v1/clearance/${id}`),
  updateStatus: (id: number, status: string) =>
    put(`/v1/clearance/${id}/status`, { status }),
  updateItem: (id: number, data: { status: string; remarks?: string }) =>
    put(`/v1/clearance/items/${id}`, data),
  listTemplates: () => get("/v1/clearance/templates"),
  createTemplate: (data: Record<string, unknown>) =>
    post("/v1/clearance/templates", data),
  deleteTemplate: (id: number) =>
    api(`/v1/clearance/templates/${id}`, { method: "DELETE" }),
};

// Final Pay
export const finalPayAPI = {
  list: (params?: Record<string, string>) => get("/v1/final-pay", params),
  get: (employeeId: number) => get(`/v1/final-pay/${employeeId}`),
  create: (data: Record<string, unknown>) => post("/v1/final-pay", data),
  updateStatus: (id: number, status: string) =>
    put(`/v1/final-pay/${id}/status`, { status }),
};

// Training
export const trainingAPI = {
  list: (params?: Record<string, string>) => get("/v1/trainings", params),
  create: (data: Record<string, unknown>) => post("/v1/trainings", data),
  updateStatus: (id: number, status: string) =>
    put(`/v1/trainings/${id}/status`, { status }),
  listParticipants: (id: number) => get(`/v1/trainings/${id}/participants`),
  addParticipant: (id: number, employee_id: number) =>
    post(`/v1/trainings/${id}/participants`, { employee_id }),
};

// Certification
export const certificationAPI = {
  list: (params?: Record<string, string>) => get("/v1/certifications", params),
  create: (data: Record<string, unknown>) => post("/v1/certifications", data),
  delete: (id: number) => api(`/v1/certifications/${id}`, { method: "DELETE" }),
  expiring: () => get("/v1/certifications/expiring"),
};

// Tax Filing & Remittance
export const taxFilingAPI = {
  list: (params?: Record<string, string>) => get("/v1/tax-filings", params),
  create: (data: Record<string, unknown>) => post("/v1/tax-filings", data),
  updateStatus: (id: number, data: Record<string, unknown>) =>
    put(`/v1/tax-filings/${id}/status`, data),
  overdue: () => get("/v1/tax-filings/overdue"),
  upcoming: () => get("/v1/tax-filings/upcoming"),
  generateAnnual: (year: number) =>
    post("/v1/tax-filings/generate-annual", { year }),
  remittances: (params?: Record<string, string>) =>
    get("/v1/tax-filings/remittances", params),
};

// Disciplinary
export const disciplinaryAPI = {
  createIncident: (data: Record<string, unknown>) =>
    post("/v1/disciplinary/incidents", data),
  listIncidents: (params?: Record<string, string>) =>
    get("/v1/disciplinary/incidents", params),
  getIncident: (id: number) => get(`/v1/disciplinary/incidents/${id}`),
  updateIncidentStatus: (
    id: number,
    data: { status: string; resolution_notes?: string },
  ) => put(`/v1/disciplinary/incidents/${id}/status`, data),
  createAction: (data: Record<string, unknown>) =>
    post("/v1/disciplinary/actions", data),
  listActions: (params?: Record<string, string>) =>
    get("/v1/disciplinary/actions", params),
  acknowledgeAction: (id: number) =>
    post(`/v1/disciplinary/actions/${id}/acknowledge`),
  appealAction: (id: number, reason: string) =>
    post(`/v1/disciplinary/actions/${id}/appeal`, { reason }),
  resolveAppeal: (id: number, data: { status: string; resolution?: string }) =>
    post(`/v1/disciplinary/actions/${id}/resolve-appeal`, data),
  getEmployeeSummary: (employeeId: number) =>
    get(`/v1/disciplinary/employee/${employeeId}/summary`),
};

// Company Policies
export const policyAPI = {
  list: () => get("/v1/policies"),
  get: (id: number) => get(`/v1/policies/${id}`),
  create: (data: Record<string, unknown>) => post("/v1/policies", data),
  update: (id: number, data: Record<string, unknown>) =>
    put(`/v1/policies/${id}`, data),
  deactivate: (id: number) => api(`/v1/policies/${id}`, { method: "DELETE" }),
  acknowledge: (id: number) => post(`/v1/policies/${id}/acknowledge`),
  listAcknowledgments: (id: number) =>
    get(`/v1/policies/${id}/acknowledgments`),
  stats: (id: number) => get(`/v1/policies/${id}/stats`),
  pending: () => get("/v1/policies/pending"),
};

// Grievance Management
export const grievanceAPI = {
  summary: () => get("/v1/grievances/summary"),
  list: (params?: Record<string, string>) => get("/v1/grievances", params),
  get: (id: number) => get(`/v1/grievances/${id}`),
  create: (data: Record<string, unknown>) => post("/v1/grievances", data),
  my: () => get("/v1/grievances/my"),
  updateStatus: (id: number, status: string) =>
    put(`/v1/grievances/${id}/status`, { status }),
  assign: (id: number, assignedTo: number) =>
    post(`/v1/grievances/${id}/assign`, { assigned_to: assignedTo }),
  resolve: (id: number, resolution: string) =>
    post(`/v1/grievances/${id}/resolve`, { resolution }),
  withdraw: (id: number) => post(`/v1/grievances/${id}/withdraw`),
  listComments: (id: number) => get(`/v1/grievances/${id}/comments`),
  addComment: (id: number, comment: string, isInternal?: boolean) =>
    post(`/v1/grievances/${id}/comments`, { comment, is_internal: isInternal }),
};

// 201 File Document Management
export const file201API = {
  listCategories: () => get("/v1/201file/categories"),
  createCategory: (data: Record<string, unknown>) =>
    post("/v1/201file/categories", data),
  listDocuments: (employeeId: number, params?: Record<string, string>) =>
    get(`/v1/201file/employee/${employeeId}`, params),
  getStats: (employeeId: number) =>
    get(`/v1/201file/employee/${employeeId}/stats`),
  upload: (employeeId: number, formData: FormData) =>
    api(`/v1/201file/employee/${employeeId}/upload`, {
      method: "POST",
      body: formData,
    }),
  download: (docId: string) => {
    const baseURL = import.meta.env.VITE_API_URL || "/api";
    const token = localStorage.getItem("token");
    return `${baseURL}/v1/201file/document/${docId}/download?token=${token}`;
  },
  updateDocument: (docId: string, data: Record<string, unknown>) =>
    put(`/v1/201file/document/${docId}`, data),
  deleteDocument: (docId: string) =>
    api(`/v1/201file/document/${docId}`, { method: "DELETE" }),
  expiring: () => get("/v1/201file/expiring"),
  compliance: (employeeId: number) =>
    get(`/v1/201file/employee/${employeeId}/compliance`),
  listRequirements: () => get("/v1/201file/requirements"),
  createRequirement: (data: Record<string, unknown>) =>
    post("/v1/201file/requirements", data),
  deleteRequirement: (id: number) =>
    api(`/v1/201file/requirements/${id}`, { method: "DELETE" }),
};

// Benefits Administration
export const benefitAPI = {
  listPlans: () => get("/v1/benefits/plans"),
  getPlan: (id: number) => get(`/v1/benefits/plans/${id}`),
  createPlan: (data: Record<string, unknown>) =>
    post("/v1/benefits/plans", data),
  updatePlan: (id: number, data: Record<string, unknown>) =>
    put(`/v1/benefits/plans/${id}`, data),
  summary: () => get("/v1/benefits/summary"),
  listEnrollments: (params?: Record<string, string>) =>
    get("/v1/benefits/enrollments", params),
  myEnrollments: () => get("/v1/benefits/my-enrollments"),
  createEnrollment: (data: Record<string, unknown>) =>
    post("/v1/benefits/enrollments", data),
  cancelEnrollment: (id: number) =>
    post(`/v1/benefits/enrollments/${id}/cancel`),
  listDependents: (enrollmentId: number) =>
    get(`/v1/benefits/enrollments/${enrollmentId}/dependents`),
  addDependent: (enrollmentId: number, data: Record<string, unknown>) =>
    post(`/v1/benefits/enrollments/${enrollmentId}/dependents`, data),
  deleteDependent: (id: number) =>
    api(`/v1/benefits/dependents/${id}`, { method: "DELETE" }),
  listClaims: (params?: Record<string, string>) =>
    get("/v1/benefits/claims", params),
  createClaim: (data: Record<string, unknown>) =>
    post("/v1/benefits/claims", data),
  approveClaim: (id: number) => post(`/v1/benefits/claims/${id}/approve`),
  rejectClaim: (id: number, reason?: string) =>
    post(`/v1/benefits/claims/${id}/reject`, { reason }),
};

// Expense Reimbursement
export const expenseAPI = {
  listCategories: () => get("/v1/expenses/categories"),
  createCategory: (data: Record<string, unknown>) =>
    post("/v1/expenses/categories", data),
  updateCategory: (id: number, data: Record<string, unknown>) =>
    put(`/v1/expenses/categories/${id}`, data),
  summary: () => get("/v1/expenses/summary"),
  list: (params?: Record<string, string>) => get("/v1/expenses", params),
  my: () => get("/v1/expenses/my"),
  get: (id: number) => get(`/v1/expenses/${id}`),
  create: (data: Record<string, unknown>) => post("/v1/expenses", data),
  submit: (id: number) => post(`/v1/expenses/${id}/submit`),
  approve: (id: number) => post(`/v1/expenses/${id}/approve`),
  reject: (id: number, reason?: string) =>
    post(`/v1/expenses/${id}/reject`, { reason }),
  markPaid: (id: number, reference?: string) =>
    post(`/v1/expenses/${id}/mark-paid`, { reference }),
};

// Geofencing
export const geofenceAPI = {
  list: () => get("/v1/attendance/geofences"),
  create: (data: Record<string, unknown>) =>
    post("/v1/attendance/geofences", data),
  update: (id: number, data: Record<string, unknown>) =>
    put(`/v1/attendance/geofences/${id}`, data),
  delete: (id: number) =>
    api(`/v1/attendance/geofences/${id}`, { method: "DELETE" }),
  getSettings: () => get("/v1/attendance/geofence-settings"),
  setSettings: (data: { geofence_enabled: boolean }) =>
    put("/v1/attendance/geofence-settings", data),
};

// Contract Milestones
export const milestoneAPI = {
  list: (params?: Record<string, string>) => get("/v1/milestones", params),
  listPending: () => get("/v1/milestones/pending"),
  acknowledge: (id: number, notes?: string) =>
    post(`/v1/milestones/${id}/acknowledge`, { notes }),
  action: (id: number, notes?: string) =>
    post(`/v1/milestones/${id}/action`, { notes }),
};

// AI Briefing
export const briefingAPI = {
  get: () => get("/v1/ai/briefing"),
};

// AI Command Palette
export const commandAPI = {
  execute: (query: string) =>
    post<{
      data: {
        result: {
          type: "action" | "query" | "info" | "navigation";
          title: string;
          message: string;
          data?: Record<string, unknown>;
          actions?: Array<{
            label: string;
            route?: string;
            action?: string;
            params?: Record<string, unknown>;
          }>;
        };
        tokens_used: number;
      };
    }>("/v1/ai/command", { query }),
};

// AI Chat
export const aiAPI = {
  chat: (data: { message: string; session_id?: string; agent?: string }) =>
    post<{
      data: {
        request_id: string;
        message: string;
        session_id: string;
        tokens_used?: number;
        agent?: string;
      };
    }>("/v1/ai/chat", data),
  streamChat: async function* (
    message: string,
    sessionId?: string,
    agentSlug?: string,
  ) {
    const baseURL = import.meta.env.VITE_API_URL || "/api";
    const token = localStorage.getItem("token");
    const response = await fetch(`${baseURL}/v1/ai/chat/stream`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        message,
        session_id: sessionId,
        agent: agentSlug,
      }),
    });

    if (response.status === 402) {
      yield {
        type: "error" as const,
        code: 402,
        message: "Insufficient token balance",
      };
      return;
    }
    if (!response.ok) {
      throw new Error(`AI chat error: ${response.status}`);
    }

    const reader = response.body?.getReader();
    if (!reader) throw new Error("No response body");

    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";

      for (const line of lines) {
        if (!line.startsWith("data: ")) continue;
        const data = line.slice(6).trim();
        if (data === "[DONE]") return;
        try {
          yield JSON.parse(data) as {
            type: "text" | "tool" | "done" | "error";
            text?: string;
            name?: string;
            message?: string;
            code?: number;
            tokens_used?: number;
            agent?: string;
            session_id?: string;
            message_id?: number;
          };
        } catch {
          // skip malformed chunks
        }
      }
    }
  },
  listAgents: () =>
    get<{
      data: Array<{
        slug: string;
        name: string;
        description: string;
        tools: string[];
        cost_multiplier: number;
        is_autonomous: boolean;
        max_rounds: number;
        icon: string;
      }>;
    }>("/v1/ai/agents"),
  getAgent: (slug: string) => get(`/v1/ai/agents/${slug}`),
  listSessions: () =>
    get<{
      data: Array<{
        id: string;
        agent_slug: string;
        title: string;
        created_at: string;
        updated_at: string;
      }>;
    }>("/v1/ai/sessions"),
  getSessionMessages: (sessionId: string) =>
    get<{
      data: Array<{
        id: number;
        role: string;
        content: string;
        tokens_used: number;
        created_at: string;
      }>;
    }>(`/v1/ai/sessions/${sessionId}/messages`),
  deleteSession: (sessionId: string) =>
    api(`/v1/ai/sessions/${sessionId}`, { method: "DELETE" }),
};

// Billing
export const billingAPI = {
  getBalance: () =>
    get<{
      data: {
        balance: number;
        total_purchased: number;
        total_granted: number;
        total_consumed: number;
      };
    }>("/v1/billing/balance"),
  listTransactions: (params?: Record<string, string>) =>
    get("/v1/billing/transactions", params),
  usageByAgent: () => get("/v1/billing/usage/agents"),
  dailyUsage: () => get("/v1/billing/usage/daily"),
  listPackages: () => get("/v1/billing/packages"),
  purchaseTokens: (packageId: number) =>
    post("/v1/billing/purchase", { package_id: packageId }),
};

// Agent API (convenience alias + CRUD)
export const agentAPI = {
  list: () => aiAPI.listAgents(),
  get: (slug: string) => aiAPI.getAgent(slug),
  create: (data: Record<string, unknown>) => post("/v1/ai/agents", data),
  update: (slug: string, data: Record<string, unknown>) =>
    put(`/v1/ai/agents/${slug}`, data),
  delete: (slug: string) => api(`/v1/ai/agents/${slug}`, { method: "DELETE" }),
  listTools: () =>
    get<{ data: Array<{ name: string; description: string }> }>(
      "/v1/ai/agents/tools",
    ),
};

// Form Prefill
export const formPrefillAPI = {
  get: (formType: string) =>
    get("/v1/ai/form-prefill", { form_type: formType }),
};

// AI Audit Log
export const aiAuditAPI = {
  list: (params?: Record<string, string>) => get("/v1/ai/audit-log", params),
};

// AI Feedback
export const feedbackAPI = {
  submit: (
    messageId: number,
    rating: "positive" | "negative",
    comment?: string,
  ) => post(`/v1/ai/messages/${messageId}/feedback`, { rating, comment }),
};

// Integrations
export const integrationAPI = {
  // Connections
  listConnections: () => get("/v1/integrations/connections"),
  getConnection: (id: string) => get(`/v1/integrations/connections/${id}`),
  createConnection: (data: Record<string, unknown>) =>
    post("/v1/integrations/connections", data),
  updateConnection: (id: string, data: Record<string, unknown>) =>
    put(`/v1/integrations/connections/${id}`, data),
  deleteConnection: (id: string) =>
    api(`/v1/integrations/connections/${id}`, { method: "DELETE" }),
  testConnection: (id: string) =>
    post(`/v1/integrations/connections/${id}/test`),
  // Templates
  listTemplates: (params?: Record<string, string>) =>
    get("/v1/integrations/templates", params),
  createTemplate: (data: Record<string, unknown>) =>
    post("/v1/integrations/templates", data),
  updateTemplate: (id: string, data: Record<string, unknown>) =>
    put(`/v1/integrations/templates/${id}`, data),
  deleteTemplate: (id: string) =>
    api(`/v1/integrations/templates/${id}`, { method: "DELETE" }),
  // Jobs
  listJobs: (params?: Record<string, string>) =>
    get("/v1/integrations/jobs", params),
  retryJob: (id: string) => post(`/v1/integrations/jobs/${id}/retry`),
  skipJob: (id: string) => post(`/v1/integrations/jobs/${id}/skip`),
  // Audit
  listAudit: (params?: Record<string, string>) =>
    get("/v1/integrations/audit", params),
  // Employee integrations
  getEmployeeIntegrations: (id: number) =>
    get(`/v1/employees/${id}/integrations`),
};

// Workflow Rules
export const workflowAPI = {
  listRules: () => get("/v1/workflow/rules"),
  createRule: (data: Record<string, unknown>) =>
    post("/v1/workflow/rules", data),
  updateRule: (id: number, data: Record<string, unknown>) =>
    put(`/v1/workflow/rules/${id}`, data),
  deactivateRule: (id: number) =>
    api(`/v1/workflow/rules/${id}`, { method: "DELETE" }),
  listExecutions: (ruleId?: number, params?: Record<string, string>) => {
    const base = ruleId
      ? `/v1/workflow/rules/${ruleId}/executions`
      : "/v1/workflow/executions";
    return get(base, params);
  },
  listSLAConfigs: () => get("/v1/workflow/sla-configs"),
  upsertSLAConfig: (data: Record<string, unknown>) =>
    put("/v1/workflow/sla-configs", data),
  getAnalytics: () => get("/v1/workflow/analytics"),
  // Triggers
  listTriggers: () => get("/v1/workflow/triggers"),
  createTrigger: (data: Record<string, unknown>) =>
    post("/v1/workflow/triggers", data),
  updateTrigger: (id: number, data: Record<string, unknown>) =>
    put(`/v1/workflow/triggers/${id}`, data),
  deactivateTrigger: (id: number) =>
    api(`/v1/workflow/triggers/${id}`, { method: "DELETE" }),
  // Decisions
  listDecisions: (params?: Record<string, string>) =>
    get("/v1/workflow/decisions", params),
  getDecision: (id: number) => get(`/v1/workflow/decisions/${id}`),
  recordOverride: (id: number, data: Record<string, unknown>) =>
    post(`/v1/workflow/decisions/${id}/override`, data),
};

// HR Service Requests
export const hrRequestAPI = {
  create: (data: Record<string, unknown>) => post("/v1/hr-requests", data),
  listMy: (params?: Record<string, string>) =>
    get("/v1/hr-requests/my", params),
  list: (params?: Record<string, string>) => get("/v1/hr-requests", params),
  get: (id: number) => get(`/v1/hr-requests/${id}`),
  updateStatus: (id: number, data: Record<string, unknown>) =>
    put(`/v1/hr-requests/${id}/status`, data),
  getStats: () => get("/v1/hr-requests/stats"),
};

// Recognition / Kudos
export const recognitionAPI = {
  send: (data: Record<string, unknown>) => post("/v1/recognitions", data),
  list: (params?: Record<string, string>) => get("/v1/recognitions", params),
  listMy: (params?: Record<string, string>) =>
    get("/v1/recognitions/my", params),
  getStats: () => get("/v1/recognitions/stats"),
};

// Pulse Surveys
export const pulseAPI = {
  list: (params?: Record<string, string>) => get("/v1/pulse-surveys", params),
  create: (data: Record<string, unknown>) => post("/v1/pulse-surveys", data),
  get: (id: number) => get(`/v1/pulse-surveys/${id}`),
  update: (id: number, data: Record<string, unknown>) =>
    put(`/v1/pulse-surveys/${id}`, data),
  deactivate: (id: number) =>
    api(`/v1/pulse-surveys/${id}`, { method: "DELETE" }),
  getResults: (id: number) => get(`/v1/pulse-surveys/${id}/results`),
  listActive: () => get("/v1/pulse-surveys/active"),
  getOpenRound: (id: number) => get(`/v1/pulse-surveys/${id}/open-round`),
  submitResponse: (roundId: number, data: Record<string, unknown>) =>
    post(`/v1/pulse-surveys/rounds/${roundId}/respond`, data),
};

// Bot
export const botAPI = {
  getLinkCode: () => get("/v1/bot/link-code"),
  getLinkStatus: () => get("/v1/bot/link-status"),
  unlinkPlatform: (platform: string) =>
    api(`/v1/bot/link/${platform}`, { method: "DELETE" }),
  // Admin
  listBotConfigs: () => get("/v1/admin/bot/configs"),
  saveBotConfig: (data: Record<string, unknown>) =>
    post("/v1/admin/bot/configs", data),
};

export const byokAPI = {
  listKeys: () => get("/v1/byok/keys"),
  createKey: (data: {
    provider: string;
    api_key: string;
    model_override?: string;
    label?: string;
    user_id?: number | null;
  }) => post("/v1/byok/keys", data),
  updateKey: (
    id: string,
    data: {
      api_key?: string;
      model_override?: string;
      label?: string;
      is_active?: boolean;
    },
  ) => api(`/v1/byok/keys/${id}`, { method: "PUT", body: data }),
  deleteKey: (id: string) => api(`/v1/byok/keys/${id}`, { method: "DELETE" }),
  validateKey: (data: { provider: string; api_key: string }) =>
    post("/v1/byok/keys/validate", data),
};
