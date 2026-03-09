export interface User {
  id: number;
  email: string;
  first_name: string;
  last_name: string;
  role: string;
  company_id: number;
}

export interface AttendanceRecord {
  id: number;
  employee_id: number;
  clock_in: string;
  clock_out: string | null;
  source: string;
  lat: string | null;
  lng: string | null;
  note: string | null;
  status: string;
  created_at: string;
}

export interface AttendanceSummary {
  today_clock_in: string | null;
  today_clock_out: string | null;
  total_hours_today: number;
  status: "not_clocked_in" | "clocked_in" | "clocked_out";
}

export interface LeaveBalance {
  leave_type_id: number;
  leave_type_name: string;
  total: number;
  used: number;
  remaining: number;
  year: number;
}

export interface LeaveType {
  id: number;
  name: string;
  max_days: number;
  description: string;
}

export interface LeaveRequest {
  id: number;
  employee_id: number;
  leave_type_id: number;
  leave_type_name?: string;
  start_date: string;
  end_date: string;
  days: number;
  reason: string;
  status: "pending" | "approved" | "rejected" | "cancelled";
  created_at: string;
}

export interface Payslip {
  id: string;
  employee_id: number;
  cycle_id: number;
  period_start: string;
  period_end: string;
  pay_date: string;
  basic_salary: number;
  gross_pay: number;
  total_deductions: number;
  net_pay: number;
  items: PayslipItem[];
}

export interface PayslipItem {
  component_name: string;
  component_type: string;
  amount: number;
}

export interface Notification {
  id: number;
  title: string;
  message: string;
  type: string;
  is_read: boolean;
  created_at: string;
}

export interface GeofenceSettings {
  geofence_enabled: boolean;
}

export interface Geofence {
  id: number;
  name: string;
  lat: number;
  lng: number;
  radius: number;
}

// AI Chat
export interface ChatSession {
  id: string;
  agent_slug: string;
  title: string;
  created_at: string;
  updated_at: string;
}

export interface Agent {
  slug: string;
  name: string;
  description: string;
  tools: string[];
  cost_multiplier: number;
  is_autonomous: boolean;
  max_rounds: number;
  icon: string;
}

export interface StreamChunk {
  type: "text" | "tool" | "done" | "error" | "confirmation";
  text?: string;
  name?: string;
  message?: string;
  code?: number;
  tokens_used?: number;
  agent?: string;
  session_id?: string;
  message_id?: number;
  // confirmation data (JSON string with DraftResult)
  data?: DraftConfirmation;
}

export interface DraftConfirmation {
  draft_id: string;
  status: string;
  tool_name: string;
  risk_level: "low" | "medium" | "high";
  description: string;
  message: string;
}

export interface ChatMessage {
  id?: number;
  role: "user" | "assistant";
  content: string;
  tokens_used?: number;
  created_at?: string;
  draft?: DraftConfirmation;
}

export interface TokenBalance {
  balance: number;
  total_purchased: number;
  total_granted: number;
  total_consumed: number;
}

export interface FormSuggestion {
  field: string;
  value: string;
  label: string;
}

export interface ApiResponse<T> {
  success: boolean;
  data: T;
  error?: string;
  meta?: {
    total: number;
    page: number;
    limit: number;
  };
}
