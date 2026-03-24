import { ref, computed } from "vue";
import { defineStore } from "pinia";
import { authAPI, companyAPI } from "../api/client";

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
  const accessToken = ref(localStorage.getItem("access_token") || "");
  const refreshToken = ref(localStorage.getItem("refresh_token") || "");
  const user = ref<User | null>(null);
  const companies = ref<
    Array<{ id: number; company_name: string; role: string }>
  >([]);
  const userLoading = ref(false);

  const isAuthenticated = computed(() => !!accessToken.value);
  const isAdmin = computed(
    () => user.value?.role === "super_admin" || user.value?.role === "admin",
  );
  const isManager = computed(
    () => isAdmin.value || user.value?.role === "manager",
  );
  const fullName = computed(() =>
    user.value ? `${user.value.first_name} ${user.value.last_name}` : "",
  );
  const jurisdiction = computed(
    () =>
      user.value?.company_country ||
      localStorage.getItem("jurisdiction") ||
      "PH",
  );
  const companyName = computed(() => {
    if (!user.value) return "";
    // Find matching company name from companies list, or derive from user info
    const match = companies.value.find((c) => c.id === user.value?.company_id);
    return match?.company_name || "";
  });

  function setTokens(access: string, refresh: string) {
    accessToken.value = access;
    refreshToken.value = refresh;
    localStorage.setItem("access_token", access);
    localStorage.setItem("refresh_token", refresh);
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
    referral_code?: string;
    jurisdiction?: string;
  }): Promise<{ emailSent: boolean }> {
    const raw = await authAPI.register(data);
    const d = (raw as Record<string, unknown>).data as
      | Record<string, unknown>
      | undefined;
    const payload = (d ?? raw) as Record<string, unknown>;

    // If email verification is required, the response has email_sent: true but no tokens
    if (payload.email_sent) {
      return { emailSent: true };
    }

    // Dev mode / no Resend: auto-login with tokens
    const res = extractAuthData(raw);
    setTokens(res.token, res.refresh_token);
    setUser(res.user);
    return { emailSent: false };
  }

  async function loginWithSSO(ssoToken: string) {
    const raw = await authAPI.ssoLogin(ssoToken);
    const res = extractAuthData(raw);
    setTokens(res.token, res.refresh_token);
    setUser(res.user);
  }

  async function fetchMe() {
    if (!accessToken.value) return;
    try {
      const raw = (await authAPI.me()) as Record<string, unknown>;
      const u = (raw.data ?? raw) as User;
      setUser(u);
    } catch {
      logout();
    }
  }

  async function fetchCompanies() {
    try {
      const raw = (await companyAPI.list()) as any;
      const data = raw?.data ?? raw;
      companies.value = Array.isArray(data) ? data : [];
    } catch {
      companies.value = [];
    }
  }

  async function switchCompany(companyId: number) {
    const raw = await authAPI.switchCompany(companyId);
    const res = extractAuthData(raw);
    setTokens(res.token, res.refresh_token);
    setUser(res.user);
    await fetchCompanies();
  }

  async function logout() {
    const refresh = localStorage.getItem("refresh_token");
    if (refresh) {
      try {
        await authAPI.logout(refresh);
      } catch {
        /* best effort */
      }
    }
    accessToken.value = "";
    refreshToken.value = "";
    user.value = null;
    companies.value = [];
    localStorage.removeItem("access_token");
    localStorage.removeItem("refresh_token");
    localStorage.removeItem("jurisdiction");
  }

  return {
    accessToken,
    user,
    companies,
    userLoading,
    isAuthenticated,
    isAdmin,
    isManager,
    fullName,
    jurisdiction,
    companyName,
    setTokens,
    setUser,
    login,
    register,
    loginWithSSO,
    fetchMe,
    fetchCompanies,
    switchCompany,
    logout,
  };
});
