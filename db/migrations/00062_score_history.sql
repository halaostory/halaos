-- +goose Up
-- Score history tables for org intelligence trend tracking

-- Flight risk history (weekly snapshots)
CREATE TABLE score_history_flight_risk (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    risk_score INT NOT NULL DEFAULT 0,
    factors JSONB NOT NULL DEFAULT '[]',
    week_date DATE NOT NULL
);
CREATE UNIQUE INDEX idx_sh_flight_risk_uq ON score_history_flight_risk(company_id, employee_id, week_date);
CREATE INDEX idx_sh_flight_risk_trend ON score_history_flight_risk(company_id, week_date);

-- Burnout history (weekly snapshots)
CREATE TABLE score_history_burnout (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    burnout_score INT NOT NULL DEFAULT 0,
    factors JSONB NOT NULL DEFAULT '[]',
    week_date DATE NOT NULL
);
CREATE UNIQUE INDEX idx_sh_burnout_uq ON score_history_burnout(company_id, employee_id, week_date);
CREATE INDEX idx_sh_burnout_trend ON score_history_burnout(company_id, week_date);

-- Team health history (weekly snapshots per department)
CREATE TABLE score_history_team_health (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    department_id BIGINT NOT NULL REFERENCES departments(id),
    department_name TEXT NOT NULL,
    health_score INT NOT NULL DEFAULT 0,
    factors JSONB NOT NULL DEFAULT '{}',
    week_date DATE NOT NULL
);
CREATE UNIQUE INDEX idx_sh_team_health_uq ON score_history_team_health(company_id, department_id, week_date);
CREATE INDEX idx_sh_team_health_trend ON score_history_team_health(company_id, week_date);

-- Org-level weekly aggregate snapshots
CREATE TABLE org_score_snapshots (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    week_date DATE NOT NULL,
    avg_flight_risk NUMERIC(5,2) NOT NULL DEFAULT 0,
    avg_burnout NUMERIC(5,2) NOT NULL DEFAULT 0,
    avg_team_health NUMERIC(5,2) NOT NULL DEFAULT 0,
    high_risk_count INT NOT NULL DEFAULT 0,
    high_burnout_count INT NOT NULL DEFAULT 0,
    low_health_dept_count INT NOT NULL DEFAULT 0,
    total_employees INT NOT NULL DEFAULT 0,
    total_departments INT NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'
);
CREATE UNIQUE INDEX idx_org_snapshots_uq ON org_score_snapshots(company_id, week_date);

-- Executive briefings (cached AI narratives)
CREATE TABLE executive_briefings (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    week_date DATE NOT NULL,
    narrative TEXT NOT NULL DEFAULT '',
    data_snapshot JSONB NOT NULL DEFAULT '{}',
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    tokens_used INT NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX idx_exec_briefings_uq ON executive_briefings(company_id, week_date);

-- +goose Down
DROP TABLE IF EXISTS executive_briefings;
DROP TABLE IF EXISTS org_score_snapshots;
DROP TABLE IF EXISTS score_history_team_health;
DROP TABLE IF EXISTS score_history_burnout;
DROP TABLE IF EXISTS score_history_flight_risk;
