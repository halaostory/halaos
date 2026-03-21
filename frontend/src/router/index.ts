import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "../stores/auth";

const router = createRouter({
  history: createWebHistory(),
  scrollBehavior(_to, _from, savedPosition) {
    return savedPosition || { top: 0 };
  },
  routes: [
    // Setup Wizard (authenticated but outside DashboardLayout)
    {
      path: "/setup",
      name: "setup-wizard",
      component: () => import("../views/SetupWizardView.vue"),
      meta: { requiresAuth: true, roles: ["super_admin", "admin"] },
    },
    // Public marketing pages (guest only)
    {
      path: "/",
      component: () => import("../components/PublicLayout.vue"),
      meta: { guestOnly: true },
      children: [
        {
          path: "",
          name: "home",
          component: () => import("../views/public/HomePage.vue"),
        },
        {
          path: "features",
          name: "features",
          component: () => import("../views/public/FeaturesPage.vue"),
        },
        {
          path: "pricing",
          name: "pricing",
          component: () => import("../views/public/PricingPage.vue"),
        },
        {
          path: "about",
          name: "about",
          component: () => import("../views/public/AboutPage.vue"),
        },
        {
          path: "contact",
          name: "contact",
          component: () => import("../views/public/ContactPage.vue"),
        },
        {
          path: "blog",
          name: "blog",
          component: () => import("../views/public/BlogPage.vue"),
        },
        {
          path: "blog/:slug",
          name: "blog-article",
          component: () => import("../views/public/BlogArticle.vue"),
        },
        {
          path: "privacy",
          name: "privacy",
          component: () => import("../views/public/PrivacyPage.vue"),
        },
        {
          path: "terms",
          name: "terms",
          component: () => import("../views/public/TermsPage.vue"),
        },
        {
          path: "tools",
          name: "tools",
          component: () => import("../views/public/ToolsPage.vue"),
        },
        {
          path: "tools/tax-calculator",
          name: "tax-calculator",
          component: () => import("../views/public/TaxCalculator.vue"),
        },
        {
          path: "tools/sss-calculator",
          name: "sss-calculator",
          component: () => import("../views/public/SSSCalculator.vue"),
        },
        {
          path: "tools/13th-month-calculator",
          name: "13th-month-calculator",
          component: () =>
            import("../views/public/ThirteenthMonthCalculator.vue"),
        },
      ],
    },
    // Auth
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
      path: "/verify-email",
      name: "verify-email",
      component: () => import("../views/VerifyEmailView.vue"),
    },
    // SSO Callback
    {
      path: "/sso",
      name: "sso",
      component: () => import("../views/SSOCallbackView.vue"),
      meta: { title: "SSO Login" },
    },
    // Dashboard (authenticated) — at root path
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
        // Org Intelligence
        {
          path: "org-intelligence",
          name: "org-intelligence",
          component: () => import("../views/OrgIntelligenceView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
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
        // Referrals
        {
          path: "referrals",
          name: "referrals",
          component: () => import("../views/ReferralsView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        // Agent Hub
        {
          path: "agent-hub",
          name: "agent-hub",
          component: () => import("../views/AgentHubView.vue"),
        },
        // Integrations
        {
          path: "integrations",
          name: "integrations",
          component: () => import("../views/IntegrationsView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "integrations/jobs",
          name: "provisioning-jobs",
          component: () => import("../views/ProvisioningJobsView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "integrations/:id",
          name: "integration-detail",
          component: () => import("../views/IntegrationDetailView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        // Workflow Rules
        {
          path: "workflow-rules",
          name: "workflow-rules",
          component: () => import("../views/WorkflowRulesView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "workflow-analytics",
          name: "workflow-analytics",
          component: () => import("../views/WorkflowAnalyticsView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        {
          path: "workflow-triggers",
          name: "workflow-triggers",
          component: () => import("../views/WorkflowTriggersView.vue"),
          meta: { roles: ["super_admin", "admin"] },
        },
        {
          path: "workflow-decisions",
          name: "workflow-decisions",
          component: () => import("../views/WorkflowDecisionsView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        // HR Requests
        {
          path: "hr-requests",
          name: "hr-requests",
          component: () => import("../views/HrRequestsView.vue"),
        },
        // Recognition
        {
          path: "recognition",
          name: "recognition",
          component: () => import("../views/RecognitionView.vue"),
        },
        // Pulse Surveys
        {
          path: "pulse-surveys",
          name: "pulse-surveys",
          component: () => import("../views/PulseSurveysView.vue"),
          meta: { roles: ["super_admin", "admin", "manager"] },
        },
        {
          path: "pulse-surveys/respond",
          name: "pulse-respond",
          component: () => import("../views/PulseRespondView.vue"),
        },
        // Notifications
        {
          path: "notifications",
          name: "notifications",
          component: () => import("../views/NotificationsView.vue"),
        },
      ],
    },
    // Backward compat: redirect old /dashboard/* paths to root
    {
      path: "/dashboard/:pathMatch(.*)*",
      redirect: (to) => to.path.replace("/dashboard", "") || "/",
    },
    // 404
    {
      path: "/:pathMatch(.*)*",
      name: "not-found",
      component: () => import("../views/NotFoundView.vue"),
    },
  ],
});

router.beforeEach(async (to) => {
  const auth = useAuthStore();

  // Auth required but not logged in
  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: "login", query: { redirect: to.fullPath } };
  }

  // Fetch user info if authenticated but no user data
  if (auth.isAuthenticated && !auth.user) {
    await auth.fetchMe();
  }

  // Role-based access control
  if (to.meta.roles && auth.user) {
    const allowed = to.meta.roles as string[];
    if (!allowed.includes(auth.user.role)) {
      return { name: "dashboard" };
    }
  }

  // Redirect authenticated users away from guest-only routes (public pages)
  if (to.meta.guestOnly && auth.isAuthenticated) {
    return { name: "dashboard" };
  }

  // Redirect authenticated users away from login/register
  if ((to.name === "login" || to.name === "register") && auth.isAuthenticated) {
    // Redirect new admins to setup wizard if not completed
    const setupDone = localStorage.getItem("halaos_setup_done");
    if (
      !setupDone &&
      (auth.user?.role === "super_admin" || auth.user?.role === "admin")
    ) {
      return { name: "setup-wizard" };
    }
    return { name: "dashboard" };
  }
});

export default router;
