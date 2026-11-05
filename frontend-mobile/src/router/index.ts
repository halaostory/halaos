import { createRouter, createWebHistory, createWebHashHistory } from "vue-router";
import { isNative } from "../platform";
import { useAuthStore } from "../stores/auth";
import { hasModule, loadEdition } from "../edition";

const router = createRouter({
  // The native app loads pages from inside the app bundle: hash routing.
  history: isNative ? createWebHashHistory() : createWebHistory("/m/"),
  routes: [
    {
      path: "/login",
      name: "login",
      component: () => import("../views/LoginView.vue"),
    },
    {
      path: "/",
      component: () => import("../components/MobileLayout.vue"),
      meta: { requiresAuth: true },
      children: [
        {
          path: "",
          name: "home",
          component: () => import("../views/HomeView.vue"),
        },
        {
          path: "attendance",
          name: "attendance",
          component: () => import("../views/AttendanceView.vue"),
        },
        {
          path: "leave",
          name: "leave",
          component: () => import("../views/LeaveView.vue"),
        },
        {
          path: "payslips",
          name: "payslips",
          component: () => import("../views/PayslipsView.vue"),
        },
        {
          path: "profile",
          name: "profile",
          component: () => import("../views/ProfileView.vue"),
        },
        {
          path: "notifications",
          name: "notifications",
          component: () => import("../views/NotificationsView.vue"),
        },
        {
          path: "expenses",
          name: "expenses",
          component: () => import("../views/ExpensesView.vue"),
        },
        {
          path: "ai-chat",
          name: "ai-chat",
          component: () => import("../views/AiChatView.vue"),
        },
      ],
    },
  ],
});

// Pages of modules this edition does not ship are not reachable.
const routeModule: Record<string, string> = { attendance: "attendance", expenses: "expenses", leave: "time_off", payslips: "payroll", "ai-chat": "ai" };

router.beforeEach(async (to) => {
  const auth = useAuthStore();
  await loadEdition();
  const mod = routeModule[String(to.name)];
  if (mod && !hasModule(mod)) return { name: "home" };

  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: "login", query: { redirect: to.fullPath } };
  }

  if (auth.isAuthenticated && !auth.user) {
    await auth.fetchMe();
  }

  if (to.name === "login" && auth.isAuthenticated) {
    return { name: "home" };
  }
});

export default router;
