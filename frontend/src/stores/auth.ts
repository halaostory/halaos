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
  company_country?: string;
  company_currency?: string;
  company_timezone?: string;
}

interface AuthResponse {
  token: string;
  refresh_token: string;
  user: User;
}

// Handle both wrapped {success, data: {...}} and direct {...} responses
function extractAuthData(raw: unknown): AuthResponse {
  const r = raw as { data?: AuthResponse } & AuthResponse;
  return (r.data ?? r) as AuthResponse;
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
    const raw = await authAPI.login({ email, password });
    const res = extractAuthData(raw);
    setTokens(res.token, res.refresh_token);
    setUser(res.user);
  }

  async function register(data: {
    company_name: string;
    email: string;
    password: string;
    first_name: string;
    last_name: string;
    country?: string;
  }) {
    const raw = await authAPI.register(data);
    const res = extractAuthData(raw);
    setTokens(res.token, res.refresh_token);
    setUser(res.user);
  }

  async function fetchMe() {
    if (!token.value) return;
    try {
      const raw = (await authAPI.me()) as Record<string, unknown>;
      const u = (raw.data ?? raw) as User;
      setUser(u);
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
