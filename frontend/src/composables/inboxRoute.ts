import type { RouteLocationRaw } from "vue-router";

export interface InboxItem {
  kind: string;
  ref_id: number;
  employee_id: number;
  employee_name: string;
  title: string;
  due_date: string;
  priority: string;
  created_at: string;
}

// Where each kind of HR inbox item gets resolved.
const kindRoute: Record<string, string> = {
  leave_approval: "approvals",
  overtime_approval: "approvals",
  attendance_correction: "approvals",
  expense_approval: "expenses",
  onboarding_task: "onboarding",
  offboarding_task: "onboarding",
  compliance_alert: "compliance",
  contract_milestone: "milestones",
  provisioning_approval: "provisioning-jobs",
  gov_filing_approval: "gov-filings",
  gov_filing_blocked: "gov-filings",
  gov_filing_failed: "gov-filings",
  gov_filing_manual: "gov-filings",
  gov_human_input: "gov-filings",
  gov_notice: "notices",
  signature_declined: "documents",
  device_action: "devices",
  final_pay_approval: "final-pay",
};

/** The page that resolves an inbox item, or null when there is none. */
export function inboxRoute(item: InboxItem): RouteLocationRaw | null {
  if (["document_expiring", "self_onboarding_review"].includes(item.kind) && item.employee_id) {
    return { name: "employee-detail", params: { id: String(item.employee_id) } };
  }
  const name = kindRoute[item.kind];
  return name ? { name } : null;
}

const formLabels: Record<string, string> = {
  bir_1601c: "1601-C", bir_1604c: "1604-C", bir_0619e: "0619-E", bir_1601eq: "1601-EQ", bir_1902: "1902",
  bir_2316: "2316", sss_r3: "R-3", sss_r1a: "R-1A", philhealth_rf1: "RF-1", philhealth_er2: "ER2",
  pagibig_mcrf: "MCRF", pagibig_registration: "registration", iras_ir8a: "IR8A", cpf_contribution: "CPF",
  irs_941: "941", us_state_registration: "state registration", lk_epf_etf: "EPF/ETF",
};
const agencyLabels: Record<string, string> = { PAGIBIG: "Pag-IBIG", PHILHEALTH: "PhilHealth" };

/** Turns "PAGIBIG pagibig_mcrf 2026-09" into "Pag-IBIG MCRF · Sep 2026". */
export function prettyTitle(title: string): string {
  let out = title.replace(/\b[a-z]+_[a-z0-9_]+\b/g, (code) => formLabels[code] ?? code);
  out = out.replace(/\b(PAGIBIG|PHILHEALTH)\b/g, (a) => agencyLabels[a] ?? a);
  return out.replace(/\b(20\d{2})-(0[1-9]|1[0-2])\b/g, (_m, y: string, mo: string) =>
    "· " + new Date(Number(y), Number(mo) - 1, 1).toLocaleString("en", { month: "short" }) + " " + y);
}
