import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "../stores/auth";

const router = createRouter({
  history: createWebHistory("/m/"),
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
          path: "ai-chat",
          name: "ai-chat",
          component: () => import("../views/AiChatView.vue"),
        },
      ],
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

  if (to.name === "login" && auth.isAuthenticated) {
    return { name: "home" };
  }
});

export default router;
