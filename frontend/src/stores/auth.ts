import { ref, computed } from "vue";
import { defineStore } from "pinia";
import { authAPI } from "../api/client";

interface User {
  id: number;
  email: string;
  first_name: string;
  last_name: string;
  role: string;
  company_id: number;
}

export const useAuthStore = defineStore("auth", () => {
  const token = ref(localStorage.getItem("token") || "");
  const refreshToken = ref(localStorage.getItem("refresh_token") || "");
  const user = ref<User | null>(null);

  const isAuthenticated = computed(() => !!token.value);
  const isAdmin = computed(
    () => user.value?.role === "super_admin" || user.value?.role === "admin",
  );
  const isManager = computed(
    () => isAdmin.value || user.value?.role === "manager",
  );
  const fullName = computed(() =>
    user.value ? `${user.value.first_name} ${user.value.last_name}` : "",
  );

  function setTokens(t: string, rt: string) {
    token.value = t;
    refreshToken.value = rt;
    localStorage.setItem("token", t);
    localStorage.setItem("refresh_token", rt);
  }

  function setUser(u: User) {
    user.value = u;
  }

  async function login(email: string, password: string) {
    const res = (await authAPI.login({ email, password })) as {
      token: string;
      refresh_token: string;
      user: User;
    };
    setTokens(res.token, res.refresh_token);
    setUser(res.user);
  }

  async function register(data: {
    company_name: string;
    email: string;
    password: string;
    first_name: string;
    last_name: string;
  }) {
    const res = (await authAPI.register(data)) as {
      token: string;
      refresh_token: string;
      user: User;
    };
    setTokens(res.token, res.refresh_token);
    setUser(res.user);
  }

  async function fetchMe() {
    if (!token.value) return;
    try {
      const res = (await authAPI.me()) as User;
      setUser(res);
    } catch {
      logout();
    }
  }

  function logout() {
    token.value = "";
    refreshToken.value = "";
    user.value = null;
    localStorage.removeItem("token");
    localStorage.removeItem("refresh_token");
  }

  return {
    token,
    user,
    isAuthenticated,
    isAdmin,
    isManager,
    fullName,
    setUser,
    login,
    register,
    fetchMe,
    logout,
  };
});
