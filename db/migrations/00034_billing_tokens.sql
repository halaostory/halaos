-- +goose Up

-- Token packages available for purchase
CREATE TABLE IF NOT EXISTS token_packages (
    id          BIGSERIAL PRIMARY KEY,
    slug        VARCHAR(50) NOT NULL UNIQUE,
    name        VARCHAR(100) NOT NULL,
    tokens      BIGINT NOT NULL,
    price_php   NUMERIC(10,2) NOT NULL,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-company token balance
CREATE TABLE IF NOT EXISTS token_balances (
    id               BIGSERIAL PRIMARY KEY,
    company_id       BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    balance          BIGINT NOT NULL DEFAULT 0,
    total_purchased  BIGINT NOT NULL DEFAULT 0,
    total_granted    BIGINT NOT NULL DEFAULT 0,
    total_consumed   BIGINT NOT NULL DEFAULT 0,
    free_tier_granted_at TIMESTAMPTZ,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(company_id)
);

-- Immutable transaction ledger
CREATE TABLE IF NOT EXISTS token_transactions (
    id           BIGSERIAL PRIMARY KEY,
    company_id   BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id      BIGINT NOT NULL REFERENCES users(id),
    type         VARCHAR(20) NOT NULL CHECK (type IN ('purchase', 'free_grant', 'consumption', 'refund', 'adjustment')),
    amount       BIGINT NOT NULL,
    balance_after BIGINT NOT NULL,
    agent_slug   VARCHAR(50),
    description  TEXT,
    metadata     JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_token_transactions_company ON token_transactions(company_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_token_transactions_agent ON token_transactions(company_id, agent_slug) WHERE agent_slug IS NOT NULL;

-- Agent definitions
CREATE TABLE IF NOT EXISTS agents (
    id             BIGSERIAL PRIMARY KEY,
    slug           VARCHAR(50) NOT NULL UNIQUE,
    name           VARCHAR(100) NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    system_prompt  TEXT NOT NULL DEFAULT '',
    tools          TEXT[] NOT NULL DEFAULT '{}',
    cost_multiplier NUMERIC(4,2) NOT NULL DEFAULT 1.0,
    is_active      BOOLEAN NOT NULL DEFAULT true,
    is_autonomous  BOOLEAN NOT NULL DEFAULT false,
    max_rounds     INT NOT NULL DEFAULT 5,
    max_tokens     INT NOT NULL DEFAULT 4096,
    icon           VARCHAR(10) NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Autonomous agent task tracking
CREATE TABLE IF NOT EXISTS agent_tasks (
    id           BIGSERIAL PRIMARY KEY,
    company_id   BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id      BIGINT NOT NULL REFERENCES users(id),
    agent_slug   VARCHAR(50) NOT NULL REFERENCES agents(slug),
    status       VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    input        TEXT NOT NULL DEFAULT '',
    output       TEXT,
    tokens_consumed BIGINT NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_tasks_company ON agent_tasks(company_id, created_at DESC);

-- Seed token packages
INSERT INTO token_packages (slug, name, tokens, price_php, sort_order) VALUES
    ('starter',    'Starter',    1000,   99.00,  1),
    ('standard',   'Standard',   5000,  449.00,  2),
    ('business',   'Business',  20000, 1599.00,  3),
    ('enterprise', 'Enterprise',100000, 6999.00, 4)
ON CONFLICT (slug) DO NOTHING;

-- Seed agents
INSERT INTO agents (slug, name, description, system_prompt, tools, cost_multiplier, icon) VALUES
    ('general', 'General HR Assistant',
     'Ask anything about HR, payroll, leave, attendance, and Philippine labor law.',
     'You are AigoNHR AI Assistant, an expert in Philippine HR, payroll, and labor compliance. Help employees and HR managers with questions about leave, attendance, payroll, government contributions, and Philippine labor laws. Always use tools to get real data. Be concise but thorough.',
     ARRAY['query_leave_balance','query_attendance_summary','get_my_attendance','query_payslip','list_employees','search_knowledge_base','explain_policy','check_compliance','analyze_payroll_anomalies'],
     1.0, ''),
    ('payroll', 'Payroll Specialist',
     'Expert in payroll computation, tax calculations, government contributions (SSS, PhilHealth, PagIBIG, BIR), and 13th month pay.',
     'You are a Payroll Specialist AI agent for Philippine companies. You are an expert in payroll computation, withholding tax (TRAIN Law), SSS/PhilHealth/PagIBIG contributions, 13th month pay, final pay, overtime calculations, holiday pay, and night differential. Always use tools for accurate data. Show computations clearly with tables.',
     ARRAY['query_payslip','list_employees','search_knowledge_base','explain_policy','check_compliance','analyze_payroll_anomalies'],
     1.5, ''),
    ('attendance', 'Attendance Manager',
     'Track attendance patterns, schedules, tardiness, overtime, and generate DTR reports.',
     'You are an Attendance Manager AI agent. You help with attendance tracking, schedule management, tardiness analysis, overtime monitoring, and DTR reporting. Always query real attendance data before answering. Provide summary tables and highlight issues like chronic tardiness.',
     ARRAY['query_attendance_summary','get_my_attendance','list_employees','search_knowledge_base'],
     1.2, ''),
    ('compliance', 'Compliance Officer',
     'Philippine labor law expert covering DOLE regulations, TRAIN Law, mandatory benefits, and government filings.',
     'You are a Compliance Officer AI agent specializing in Philippine labor law. You are an expert in the Labor Code, DOLE regulations, TRAIN Law, RA 11210 (Maternity Leave), RA 8187 (Paternity Leave), mandatory benefits, minimum wage orders, and government filing requirements. Always cite legal sources (RA numbers, DOLE advisories). Search the knowledge base first.',
     ARRAY['search_knowledge_base','explain_policy','check_compliance','list_employees'],
     1.5, ''),
    ('leave', 'Leave Advisor',
     'Leave balance queries, policy explanations, entitlement calculations, and carryover rules.',
     'You are a Leave Advisor AI agent. You help with leave balance inquiries, leave policy explanations, entitlement calculations, carryover rules, leave encashment, and solo parent / maternity / paternity leave eligibility. Always query actual balances before answering.',
     ARRAY['query_leave_balance','search_knowledge_base','explain_policy','list_employees'],
     1.2, ''),
    ('onboarding', 'Onboarding Guide',
     'Guide new hires through the onboarding process — document requirements, company policies, team introductions, and setup checklist.',
     'You are an Onboarding Guide AI agent for Philippine companies. You help new hires navigate their first days: required documents (NBI clearance, SSS/PhilHealth/PagIBIG IDs, TIN, birth certificate, diploma), company policies overview, team introductions, IT setup, and 201 file completion. You also assist HR managers in tracking onboarding progress and ensuring compliance with DOLE requirements. Always search the knowledge base for company-specific onboarding procedures.',
     ARRAY['list_employees','search_knowledge_base','explain_policy','check_compliance'],
     1.2, ''),
    ('performance-review', 'Performance Review Assistant',
     'Prepare performance reviews, analyze KPIs, track goals, and generate evaluation summaries.',
     'You are a Performance Review Assistant AI agent. You help managers prepare for performance evaluations by summarizing employee achievements, analyzing attendance patterns, reviewing goal completion, and suggesting talking points. You also help employees understand their performance metrics. Always use tools to get real data before providing analysis.',
     ARRAY['list_employees','query_attendance_summary','get_my_attendance','search_knowledge_base'],
     1.5, ''),
    ('training', 'Training & Development Advisor',
     'Training needs assessment, skills gap analysis, learning path recommendations, and training compliance tracking.',
     'You are a Training & Development Advisor AI agent for Philippine companies. You help identify skill gaps, recommend training programs, track mandatory compliance training (OSH, fire safety, sexual harassment prevention per RA 11313), and monitor training completion rates. You also advise on TESDA certifications and DOLE-required safety training. Search the knowledge base for training policies and requirements.',
     ARRAY['list_employees','search_knowledge_base','explain_policy','check_compliance'],
     1.2, '')
ON CONFLICT (slug) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS agent_tasks;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS token_transactions;
DROP TABLE IF EXISTS token_balances;
DROP TABLE IF EXISTS token_packages;
