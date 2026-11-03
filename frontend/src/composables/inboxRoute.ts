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
