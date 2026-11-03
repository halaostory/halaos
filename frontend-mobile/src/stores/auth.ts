import { ref, computed } from "vue";
import { defineStore } from "pinia";
import { authAPI } from "../api/client";
import type { User } from "../types";

interface AuthResponse {
  token: string;
  refresh_token: string;
  user: User;
}

function extractAuthData(raw: unknown): AuthResponse {
  const r = raw as { data?: AuthResponse } & AuthResponse;
  return (r.data ?? r) as AuthResponse;
}

export const useAuthStore = defineStore("auth", () => {
  const token = ref(localStorage.getItem("token") || "");
  const refreshToken = ref(localStorage.getItem("refresh_token") || "");
  const user = ref<User | null>(null);

  const isAuthenticated = computed(() => !!token.value);
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

  /** Returns an MFA challenge token when the account uses two-factor sign-in. */
  async function login(email: string, password: string): Promise<{ mfaToken?: string }> {
    const raw = await authAPI.login({ email, password });
    const res = extractAuthData(raw) as ReturnType<typeof extractAuthData> & { mfa_required?: boolean; mfa_token?: string };
    if (res.mfa_required && res.mfa_token) return { mfaToken: res.mfa_token };
    setTokens(res.token, res.refresh_token);
    setUser(res.user);
    return {};
  }

  async function verifyMfa(mfaToken: string, code: string) {
    const res = extractAuthData(await authAPI.mfaVerify({ mfa_token: mfaToken, code }));
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
    fullName,
    setUser,
    login,
    verifyMfa,
    fetchMe,
    logout,
  };
});
