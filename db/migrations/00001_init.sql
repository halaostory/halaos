-- +goose Up

-- Companies
CREATE TABLE companies (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    legal_name VARCHAR(255),
    tin VARCHAR(20),
    bir_rdo VARCHAR(10),
    address TEXT,
    city VARCHAR(100),
    province VARCHAR(100),
    zip_code VARCHAR(10),
    country VARCHAR(3) NOT NULL DEFAULT 'PHL',
    timezone VARCHAR(50) NOT NULL DEFAULT 'Asia/Manila',
    currency VARCHAR(3) NOT NULL DEFAULT 'PHP',
    pay_frequency VARCHAR(20) NOT NULL DEFAULT 'semi_monthly', -- monthly, semi_monthly, weekly
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    logo_url VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users (login accounts)
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'employee', -- super_admin, admin, manager, employee
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- active, inactive, suspended
    avatar_url VARCHAR(500),
    locale VARCHAR(10) NOT NULL DEFAULT 'en',
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, email)
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_company_id ON users(company_id);

-- Departments
CREATE TABLE departments (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    code VARCHAR(20) NOT NULL,
    name VARCHAR(255) NOT NULL,
    parent_id BIGINT REFERENCES departments(id),
    head_employee_id BIGINT, -- will FK to employees later
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, code)
);

CREATE INDEX idx_departments_company ON departments(company_id);

-- Positions
CREATE TABLE positions (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    code VARCHAR(20) NOT NULL,
    title VARCHAR(255) NOT NULL,
    department_id BIGINT REFERENCES departments(id),
    grade VARCHAR(10),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, code)
);

CREATE INDEX idx_positions_company ON positions(company_id);

-- Cost Centers
CREATE TABLE cost_centers (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    code VARCHAR(20) NOT NULL,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, code)
);

-- +goose Down
DROP TABLE IF EXISTS cost_centers;
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS departments;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS companies;
