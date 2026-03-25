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
      const url =
        typeof request === "string" ? request : request?.toString() || "";
      if (url.includes("/auth/login")) return;
      if (url.includes("/auth/refresh")) {
        localStorage.removeItem("token");
        localStorage.removeItem("refresh_token");
        window.location.href = "/m/login";
        return;
      }

      if (!isRefreshing) {
        isRefreshing = true;
        refreshPromise = tryRefreshToken().finally(() => {
          isRefreshing = false;
          refreshPromise = null;
        });
      }

      const refreshed = await refreshPromise;
      if (refreshed) {
        const newToken = localStorage.getItem("token");
        const headers = new Headers(options.headers as HeadersInit);
        headers.set("Authorization", `Bearer ${newToken}`);
        await ofetch(request, { ...options, headers });
        return;
      }

      localStorage.removeItem("token");
      localStorage.removeItem("refresh_token");
      window.location.href = "/m/login";
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
  login: (data: { email: string; password: string }) =>
    post("/v1/auth/login", data),
  me: () => get("/v1/auth/me"),
  changePassword: (data: { current_password: string; new_password: string }) =>
    put("/v1/auth/password", data),
  updateProfile: (data: {
    first_name: string;
    last_name: string;
    locale?: string;
  }) => put("/v1/auth/profile", data),
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
};

// Leave
export const leaveAPI = {
  listTypes: () => get("/v1/leaves/types"),
  getBalances: () => get("/v1/leaves/balances"),
  listRequests: (params?: Record<string, string>) =>
    get("/v1/leaves/requests", params),
  createRequest: (data: Record<string, unknown>) =>
    post("/v1/leaves/requests", data),
  cancelRequest: (id: number) => post(`/v1/leaves/requests/${id}/cancel`),
};

// Payroll
export const payrollAPI = {
  listPayslips: (params?: Record<string, string>) =>
    get("/v1/payroll/payslips", params),
  getPayslip: (id: string) => get(`/v1/payroll/payslips/${id}`),
  payslipPdfUrl: (id: string) =>
    `${import.meta.env.VITE_API_URL || "/api"}/v1/payroll/payslips/${id}/pdf`,
};

// Self-Service
export const selfServiceAPI = {
  getMyInfo: () => get("/v1/self-service/info"),
};

// Notifications
export const notificationAPI = {
  list: () => get("/v1/notifications"),
  unreadCount: () =>
    get<{ data: { count: number } }>("/v1/notifications/unread-count"),
  markRead: (id: number) => post(`/v1/notifications/${id}/read`),
  markAllRead: () => post("/v1/notifications/read-all"),
};

// Dashboard
export const dashboardAPI = {
  getStats: () => get("/v1/dashboard/stats"),
  getAttendance: () => get("/v1/dashboard/attendance"),
  getLeaveSummary: () => get("/v1/dashboard/leave-summary"),
};

// Geofence
export const geofenceAPI = {
  list: () => get("/v1/attendance/geofences"),
  getSettings: () => get("/v1/attendance/geofence-settings"),
};

// AI Chat
function getBaseURL() {
  return import.meta.env.VITE_API_URL || "/api";
}

function getToken() {
  return localStorage.getItem("token");
}

export const aiAPI = {
  streamChat: async function* (
    message: string,
    sessionId?: string,
    agentSlug?: string,
    pageContext?: { section: string; action?: string },
  ) {
    const baseURL = getBaseURL();
    const token = getToken();
    const body: Record<string, unknown> = {
      message,
      session_id: sessionId,
      agent: agentSlug,
    };
    if (pageContext) {
      body.page_context = pageContext;
    }
    const response = await fetch(`${baseURL}/v1/ai/chat/stream`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify(body),
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
      yield {
        type: "error" as const,
        code: response.status,
        message: `AI chat error: ${response.status}`,
      };
      return;
    }

    const reader = response.body?.getReader();
    if (!reader) return;

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
          yield JSON.parse(data);
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
  confirmDraft: (draftId: string) => post(`/v1/ai/drafts/${draftId}/confirm`),
  rejectDraft: (draftId: string) => post(`/v1/ai/drafts/${draftId}/reject`),
  submitFeedback: (messageId: number, rating: "positive" | "negative") =>
    post(`/v1/ai/messages/${messageId}/feedback`, { rating }),
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
};

// Form Prefill
export const formPrefillAPI = {
  get: (formType: string) =>
    get("/v1/ai/form-prefill", { form_type: formType }),
};

// Bot
export const botAPI = {
  getLinkCode: () => get("/v1/bot/link-code"),
  getLinkStatus: () => get("/v1/bot/link-status"),
  unlinkPlatform: (platform: string) =>
    api(`/v1/bot/link/${platform}`, { method: "DELETE" }),
};

// Onboarding Checklist
export const onboardingChecklistAPI = {
  getProgress: (persona = "employee") =>
    get("/v1/onboarding-checklist/my-progress", { persona }),
  completeStep: (step: string, persona = "employee") =>
    post("/v1/onboarding-checklist/complete-step", { step, persona }),
  dismiss: (persona = "employee") =>
    post("/v1/onboarding-checklist/dismiss", { persona }),
};
