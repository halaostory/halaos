import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "../stores/auth";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/login",
      name: "login",
      component: () => import("../views/LoginView.vue"),
    },
    {
      path: "/register",
      name: "register",
      component: () => import("../views/RegisterView.vue"),
    },
    {
      path: "/",
      component: () => import("../components/DashboardLayout.vue"),
      meta: { requiresAuth: true },
      children: [
        {
          path: "",
          name: "dashboard",
          component: () => import("../views/DashboardView.vue"),
        },
        // Employees
        {
          path: "employees",
          name: "employees",
          component: () => import("../views/EmployeesView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        {
          path: "employees/new",
          name: "employee-new",
          component: () => import("../views/EmployeeFormView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "employees/:id",
          name: "employee-detail",
          component: () => import("../views/EmployeeDetailView.vue"),
        },
        {
          path: "employees/:id/edit",
          name: "employee-edit",
          component: () => import("../views/EmployeeFormView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        // Directory
        {
          path: "directory",
          name: "directory",
          component: () => import("../views/DirectoryView.vue"),
        },
        // Attendance
        {
          path: "attendance",
          name: "attendance",
          component: () => import("../views/AttendanceView.vue"),
        },
        {
          path: "attendance/records",
          name: "attendance-records",
          component: () => import("../views/AttendanceRecordsView.vue"),
        },
        {
          path: "attendance/report",
          name: "attendance-report",
          component: () => import("../views/AttendanceReportView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        {
          path: "dtr",
          name: "dtr",
          component: () => import("../views/DTRView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        // Leave
        {
          path: "leaves",
          name: "leaves",
          component: () => import("../views/LeavesView.vue"),
        },
        {
          path: "leave-calendar",
          name: "leave-calendar",
          component: () => import("../views/LeaveCalendarView.vue"),
        },
        {
          path: "leave-encashment",
          name: "leave-encashment",
          component: () => import("../views/LeaveEncashmentView.vue"),
        },
        // Overtime
        {
          path: "overtime",
          name: "overtime",
          component: () => import("../views/OvertimeView.vue"),
        },
        // Approvals
        {
          path: "approvals",
          name: "approvals",
          component: () => import("../views/ApprovalsView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        // Payroll
        {
          path: "payroll",
          name: "payroll",
          component: () => import("../views/PayrollView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "payslips",
          name: "payslips",
          component: () => import("../views/PayslipsView.vue"),
        },
        // Onboarding
        {
          path: "onboarding",
          name: "onboarding",
          component: () => import("../views/OnboardingView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        // Schedules
        {
          path: "schedules",
          name: "schedules",
          component: () => import("../views/SchedulesView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        // Loans
        {
          path: "loans",
          name: "loans",
          component: () => import("../views/LoansView.vue"),
        },
        // Analytics
        {
          path: "analytics",
          name: "analytics",
          component: () => import("../views/AnalyticsView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        // Self-Service
        {
          path: "self-service",
          name: "self-service",
          component: () => import("../views/SelfServiceView.vue"),
        },
        // Performance
        {
          path: "performance",
          name: "performance",
          component: () => import("../views/PerformanceView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        // Knowledge Base
        {
          path: "knowledge",
          name: "knowledge",
          component: () => import("../views/KnowledgeView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        // Audit Trail
        {
          path: "audit",
          name: "audit",
          component: () => import("../views/AuditView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        // Final Pay
        {
          path: "final-pay",
          name: "final-pay",
          component: () => import("../views/FinalPayView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "clearance",
          name: "clearance",
          component: () => import("../views/ClearanceView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        // Disciplinary
        {
          path: "disciplinary",
          name: "disciplinary",
          component: () => import("../views/DisciplinaryView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        // Contract Milestones
        {
          path: "milestones",
          name: "milestones",
          component: () => import("../views/MilestonesView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        // Training & Certification
        {
          path: "training",
          name: "training",
          component: () => import("../views/TrainingView.vue"),
        },
        // Import / Export
        {
          path: "import-export",
          name: "import-export",
          component: () => import("../views/ImportExportView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        // Expenses
        {
          path: "expenses",
          name: "expenses",
          component: () => import("../views/ExpensesView.vue"),
        },
        // Geofencing
        {
          path: "geofences",
          name: "geofences",
          component: () => import("../views/GeofenceView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        // Settings
        {
          path: "departments",
          name: "departments",
          component: () => import("../views/DepartmentsView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "positions",
          name: "positions",
          component: () => import("../views/PositionsView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "salary",
          name: "salary",
          component: () => import("../views/SalaryConfigView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "compliance",
          name: "compliance",
          component: () => import("../views/ComplianceView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "tax-filings",
          name: "tax-filings",
          component: () => import("../views/TaxFilingView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "benefits",
          name: "benefits",
          component: () => import("../views/BenefitsView.vue"),
        },
        {
          path: "policies",
          name: "policies",
          component: () => import("../views/PolicyView.vue"),
        },
        {
          path: "grievance",
          name: "grievance",
          component: () => import("../views/GrievanceView.vue"),
        },
        {
          path: "201file",
          name: "201file",
          component: () => import("../views/File201View.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        {
          path: "holidays",
          name: "holidays",
          component: () => import("../views/HolidaysView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "announcements",
          name: "announcements",
          component: () => import("../views/AnnouncementsView.vue"),
        },
        {
          path: "users",
          name: "users",
          component: () => import("../views/UsersView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "settings",
          name: "settings",
          component: () => import("../views/SettingsView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "profile",
          name: "profile",
          component: () => import("../views/ProfileView.vue"),
        },
        // Billing
        {
          path: "billing",
          name: "billing",
          component: () => import("../views/BillingView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        // Agent Hub
        {
          path: "agent-hub",
          name: "agent-hub",
          component: () => import("../views/AgentHubView.vue"),
        },
        // Notifications
        {
          path: "notifications",
          name: "notifications",
          component: () => import("../views/NotificationsView.vue"),
        },
      ],
    },
    {
      path: "/:pathMatch(.*)*",
      name: "not-found",
      component: () => import("../views/NotFoundView.vue"),
    },
  ],
});

router.beforeEach(async (to) => {
  const auth = useAuthStore();

  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: "login", query: { redirect: to.fullPath } };
  }

  if (auth.isAuthenticated && !auth.user) {
    await auth.fetchMe();
  }

  if (to.meta.roles && auth.user) {
    const allowed = to.meta.roles as string[];
    if (!allowed.includes(auth.user.role)) {
      return { name: "dashboard" };
    }
  }

  if ((to.name === "login" || to.name === "register") && auth.isAuthenticated) {
    return { name: "dashboard" };
  }
});

export default router;
