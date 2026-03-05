-- +goose Up

-- Employees (HR record, may or may not have a user login)
CREATE TABLE employees (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT REFERENCES users(id),
    employee_no VARCHAR(20) NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    middle_name VARCHAR(100),
    suffix VARCHAR(20), -- Jr, Sr, III
    display_name VARCHAR(255),
    email VARCHAR(255),
    phone VARCHAR(20),
    birth_date DATE,
    gender VARCHAR(10),
    civil_status VARCHAR(20),
    nationality VARCHAR(50) DEFAULT 'Filipino',
    department_id BIGINT REFERENCES departments(id),
    position_id BIGINT REFERENCES positions(id),
    cost_center_id BIGINT REFERENCES cost_centers(id),
    manager_id BIGINT REFERENCES employees(id),
    hire_date DATE NOT NULL,
    regularization_date DATE,
    separation_date DATE,
    employment_type VARCHAR(20) NOT NULL DEFAULT 'regular', -- regular, probationary, contractual, part_time
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- draft, active, on_leave, separated
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, employee_no)
);

CREATE INDEX idx_employees_company ON employees(company_id);
CREATE INDEX idx_employees_department ON employees(department_id);
CREATE INDEX idx_employees_manager ON employees(manager_id);
CREATE INDEX idx_employees_status ON employees(company_id, status);
CREATE INDEX idx_employees_user ON employees(user_id);

-- Employee Profiles (sensitive PII)
CREATE TABLE employee_profiles (
    employee_id BIGINT PRIMARY KEY REFERENCES employees(id),
    -- Address
    address_line1 VARCHAR(255),
    address_line2 VARCHAR(255),
    city VARCHAR(100),
    province VARCHAR(100),
    zip_code VARCHAR(10),
    -- Emergency Contact
    emergency_name VARCHAR(255),
    emergency_phone VARCHAR(20),
    emergency_relation VARCHAR(50),
    -- Bank Info
    bank_name VARCHAR(100),
    bank_account_no VARCHAR(50),
    bank_account_name VARCHAR(255),
    -- Government IDs (sensitive)
    tin VARCHAR(20),
    sss_no VARCHAR(20),
    philhealth_no VARCHAR(20),
    pagibig_no VARCHAR(20),
    -- Other
    blood_type VARCHAR(5),
    religion VARCHAR(50),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Employee Documents
CREATE TABLE employee_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    doc_type VARCHAR(50) NOT NULL, -- contract, id_photo, gov_id, certificate, etc.
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_size BIGINT NOT NULL DEFAULT 0,
    mime_type VARCHAR(100),
    file_hash VARCHAR(64), -- SHA-256 for integrity
    uploaded_by BIGINT REFERENCES users(id),
    verified_at TIMESTAMPTZ,
    verified_by BIGINT REFERENCES users(id),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_employee_docs_employee ON employee_documents(employee_id);

-- Employment History (promotions, transfers)
CREATE TABLE employment_history (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    action_type VARCHAR(30) NOT NULL, -- hire, promotion, transfer, salary_change, separation
    effective_date DATE NOT NULL,
    from_department_id BIGINT REFERENCES departments(id),
    to_department_id BIGINT REFERENCES departments(id),
    from_position_id BIGINT REFERENCES positions(id),
    to_position_id BIGINT REFERENCES positions(id),
    remarks TEXT,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_employment_history_employee ON employment_history(employee_id, effective_date DESC);

-- Add FK from departments.head_employee_id to employees
ALTER TABLE departments ADD CONSTRAINT fk_dept_head FOREIGN KEY (head_employee_id) REFERENCES employees(id);

-- +goose Down
ALTER TABLE departments DROP CONSTRAINT IF EXISTS fk_dept_head;
DROP TABLE IF EXISTS employment_history;
DROP TABLE IF EXISTS employee_documents;
DROP TABLE IF EXISTS employee_profiles;
DROP TABLE IF EXISTS employees;
