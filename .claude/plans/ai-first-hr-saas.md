# AI-First HR SaaS for Philippine Market - Implementation Plan

## Task Type
- [x] Backend (Go AI service layer, worker, LLM providers)
- [x] Frontend (Chat UI, AI dashboard, streaming)
- [x] Fullstack (End-to-end AI integration)

## Architecture
`Handler -> AI Orchestrator -> IntentRouter -> ToolExecutor + ProviderClient -> AuditWriter`

## Phases

### Phase 0: AI Foundations
1. LLM provider abstraction (Anthropic primary, OpenAI/Gemini fallback)
2. PII redaction middleware (TIN, SSS, PhilHealth, PagIBIG, bank)
3. AI audit logging on every call
4. Background worker for async AI tasks

### Phase 1: AI HR Assistant Chat
1. POST /ai/chat + GET /ai/chat/stream (SSE)
2. HR tools: query_leave_balance, query_attendance, get_payslip, check_compliance
3. Frontend floating chat panel with markdown rendering

### Phase 2: Smart Dashboard + Payroll Intelligence
1. AI insight cards on Dashboard
2. Payroll anomaly detection via hr_events
3. Compliance warnings

### Phase 3: PH Compliance Automation
1. Auto-generate BIR 2316, SSS R-3, PhilHealth RF1
2. DOLE compliance checker
3. 13th month pay + final pay calculators

### Phase 4: Advanced
1. Natural language search
2. Tagalog support
3. PH holiday calendar
4. Mobile-responsive chat

## SESSION_ID
- CODEX_SESSION: 019cbe8e-82ef-7a91-a46e-52e9424d6d08
