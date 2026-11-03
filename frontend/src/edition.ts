// HalaPeople editions. Must match internal/edition.
//   people — default: payroll, compliance, lifecycle, time off, benefits, expenses, contractors, e-sign, IT, AI
//   suite  — people + attendance, talent, engagement and people analytics
import { reactive } from "vue";

export interface EditionCountry {
  code: string; // ISO 3166-1 alpha-3 (PHL, SGP …)
  name: string;
  status: "live" | "coming_soon";
}

export interface EditionConfig {
  edition: "people" | "suite";
  product_name: string;
  tagline: string;
  modules: string[];
  countries: EditionCountry[];
  ai_billing?: boolean;
  ai_assistant?: boolean;
}

// Route / nav key → module. Anything not listed belongs to "core".
const routeModule: Record<string, string> = {
  employees: "people", "employee-new": "people", "employee-detail": "people", "employee-edit": "people",
  directory: "people", "201file": "people", "custom-fields": "people", workplaces: "people",
  hire: "lifecycle", inbox: "lifecycle", onboarding: "lifecycle", clearance: "lifecycle", milestones: "lifecycle",
  payroll: "payroll", payslips: "payroll", loans: "payroll", "final-pay": "payroll", salary: "payroll", accounting: "payroll",
  compliance: "compliance", "tax-filings": "compliance", "gov-filings": "compliance", notices: "compliance",
  leaves: "time_off", "leave-calendar": "time_off", "leave-encashment": "time_off", overtime: "time_off",
  benefits: "benefits",
  expenses: "expenses",
  contractors: "contractors", "contractor-form": "contractors",
  documents: "esign", "onboard-portal": "esign", "sign-document": "esign",
  devices: "it", integrations: "it", "provisioning-jobs": "it", "integration-detail": "it",
  "agent-hub": "ai", "bot-setup": "ai", routines: "ai",
  attendance: "attendance", "attendance-records": "attendance", "attendance-report": "attendance", dtr: "attendance",
  schedules: "attendance", geofences: "attendance",
  performance: "talent", training: "talent", disciplinary: "talent", grievance: "talent",
  "pulse-surveys": "engagement", "pulse-respond": "engagement", recognition: "engagement",
  analytics: "intelligence", insights: "intelligence", "org-intelligence": "intelligence", knowledge: "intelligence",
  "workflow-rules": "intelligence", "workflow-analytics": "intelligence", "workflow-triggers": "intelligence",
  "workflow-decisions": "intelligence",
};

const fallback: EditionConfig = {
  edition: "people",
  product_name: "HalaPeople",
  tagline: "Payroll, compliance and HR for Southeast Asia — on autopilot",
  modules: [],
  countries: [],
};

export const edition = reactive<{ config: EditionConfig; loaded: boolean }>({ config: fallback, loaded: false });

let loading: Promise<void> | null = null;

/** Loads the product configuration from the API once (falls back to HalaPeople with every module). */
export function loadEdition(): Promise<void> {
  if (!loading) {
    loading = fetch((import.meta.env.VITE_API_URL || "/api") + "/v1/edition", { signal: AbortSignal.timeout(4000) })
      .then((r) => (r.ok ? r.json() : null))
      .then((j) => {
        if (j?.data?.edition) edition.config = j.data as EditionConfig;
      })
      .catch(() => undefined)
      .finally(() => {
        edition.loaded = true;
        document.title = edition.config.product_name + " — " + edition.config.tagline;
      });
  }
  return loading;
}

export function moduleOf(key: string): string {
  return routeModule[key] || "core";
}

/** Whether a route name / nav key is part of the current product. */
export function featureEnabled(key: string): boolean {
  const mods = edition.config.modules;
  if (!mods.length) return true; // not loaded yet or full edition
  return mods.includes(moduleOf(key));
}

