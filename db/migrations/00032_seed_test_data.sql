-- +goose Up
-- =============================================================================
-- COMPREHENSIVE TEST SEED DATA
-- Populates realistic test data for company_id=1 (Demo Company)
-- All data uses ON CONFLICT / WHERE NOT EXISTS for idempotency
-- =============================================================================

-- =====================================================================
-- 1. COMPANY & ADMIN USER (ensure base records exist)
-- =====================================================================
INSERT INTO companies (id, name, legal_name, tin, bir_rdo, address, city, province, zip_code, country, timezone, currency, pay_frequency, status, geofence_enabled)
VALUES (1, 'Demo Company', 'Demo Company Inc.', '123-456-789-000', '044', '123 Rizal Avenue, Brgy. San Miguel', 'Makati City', 'Metro Manila', '1200', 'PHL', 'Asia/Manila', 'PHP', 'semi_monthly', 'active', true)
ON CONFLICT (id) DO UPDATE SET
    legal_name = EXCLUDED.legal_name,
    tin = EXCLUDED.tin,
    bir_rdo = EXCLUDED.bir_rdo,
    address = EXCLUDED.address,
    city = EXCLUDED.city,
    province = EXCLUDED.province,
    zip_code = EXCLUDED.zip_code,
    geofence_enabled = EXCLUDED.geofence_enabled;

-- Admin user (password: "password123")
INSERT INTO users (id, company_id, email, password_hash, first_name, last_name, role, status)
VALUES (1, 1, 'admin@demo.com', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Admin', 'User', 'admin', 'active')
ON CONFLICT (id) DO UPDATE SET password_hash = EXCLUDED.password_hash;

-- =====================================================================
-- 2. DEPARTMENTS
-- =====================================================================
INSERT INTO departments (id, company_id, code, name, is_active) VALUES
    (1, 1, 'ENG',   'Engineering',       true),
    (2, 1, 'HR',    'Human Resources',   true),
    (3, 1, 'FIN',   'Finance',           true),
    (4, 1, 'MKT',   'Marketing',         true),
    (5, 1, 'OPS',   'Operations',        true),
    (6, 1, 'ADMIN', 'Administration',    true),
    (7, 1, 'SALES', 'Sales',             true)
ON CONFLICT DO NOTHING;

-- Reset sequence
SELECT setval('departments_id_seq', GREATEST((SELECT MAX(id) FROM departments), 7));

-- =====================================================================
-- 3. POSITIONS
-- =====================================================================
INSERT INTO positions (id, company_id, code, title, department_id, grade, is_active) VALUES
    (1,  1, 'CTO',       'Chief Technology Officer',  1, 'E1', true),
    (2,  1, 'SR-DEV',    'Senior Software Developer', 1, 'M2', true),
    (3,  1, 'JR-DEV',    'Junior Software Developer', 1, 'J1', true),
    (4,  1, 'QA-LEAD',   'QA Lead',                   1, 'M1', true),
    (5,  1, 'HR-MGR',    'HR Manager',                2, 'M2', true),
    (6,  1, 'HR-SPEC',   'HR Specialist',             2, 'J2', true),
    (7,  1, 'FIN-MGR',   'Finance Manager',           3, 'M2', true),
    (8,  1, 'ACCT',      'Accountant',                3, 'J2', true),
    (9,  1, 'MKT-MGR',   'Marketing Manager',         4, 'M2', true),
    (10, 1, 'MKT-ASSOC', 'Marketing Associate',       4, 'J1', true),
    (11, 1, 'OPS-MGR',   'Operations Manager',        5, 'M2', true),
    (12, 1, 'OPS-COORD', 'Operations Coordinator',    5, 'J1', true),
    (13, 1, 'ADMIN-ASST','Administrative Assistant',   6, 'J1', true),
    (14, 1, 'SALES-REP', 'Sales Representative',      7, 'J1', true)
ON CONFLICT DO NOTHING;

SELECT setval('positions_id_seq', GREATEST((SELECT MAX(id) FROM positions), 14));

-- =====================================================================
-- 4. USERS (login accounts for employees who need system access)
-- All passwords: "password123"
-- =====================================================================
INSERT INTO users (id, company_id, email, password_hash, first_name, last_name, role, status) VALUES
    (2,  1, 'reymond.santos@demo.com',   '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Reymond',   'Santos',       'manager',  'active'),
    (3,  1, 'maria.reyes@demo.com',       '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Maria',     'Reyes',        'manager',  'active'),
    (4,  1, 'jose.garcia@demo.com',       '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Jose',      'Garcia',       'manager',  'active'),
    (5,  1, 'ana.cruz@demo.com',          '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Ana',       'Cruz',         'manager',  'active'),
    (6,  1, 'miguel.fernandez@demo.com',  '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Miguel',    'Fernandez',    'manager',  'active'),
    (7,  1, 'carlo.mendoza@demo.com',     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Carlo',     'Mendoza',      'employee', 'active'),
    (8,  1, 'jasmine.villanueva@demo.com','$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Jasmine',   'Villanueva',   'employee', 'active'),
    (9,  1, 'mark.aquino@demo.com',       '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Mark',      'Aquino',       'employee', 'active'),
    (10, 1, 'patricia.bautista@demo.com', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Patricia',  'Bautista',     'employee', 'active'),
    (11, 1, 'rafael.dizon@demo.com',      '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Rafael',    'Dizon',        'employee', 'active'),
    (12, 1, 'grace.soriano@demo.com',     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Grace',     'Soriano',      'employee', 'active'),
    (13, 1, 'kevin.delos-reyes@demo.com', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Kevin',     'Delos Reyes',  'employee', 'active'),
    (14, 1, 'cristina.lim@demo.com',      '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Cristina',  'Lim',          'employee', 'active'),
    (15, 1, 'daniel.pascual@demo.com',    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Daniel',    'Pascual',      'employee', 'active'),
    (16, 1, 'angelica.navarro@demo.com',  '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Angelica',  'Navarro',      'employee', 'active'),
    (17, 1, 'rico.santiago@demo.com',     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Rico',      'Santiago',     'employee', 'active'),
    (18, 1, 'diana.ramos@demo.com',       '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Diana',     'Ramos',        'employee', 'active')
ON CONFLICT (id) DO NOTHING;

SELECT setval('users_id_seq', GREATEST((SELECT MAX(id) FROM users), 18));

-- =====================================================================
-- 5. EMPLOYEES (18 total, mix of statuses and types)
-- =====================================================================
INSERT INTO employees (id, company_id, user_id, employee_no, first_name, last_name, middle_name, email, phone, birth_date, gender, civil_status, nationality, department_id, position_id, manager_id, hire_date, regularization_date, separation_date, employment_type, status, contract_end_date) VALUES
    -- Engineering Department
    (1,  1, 2,  'EMP-2022-001', 'Reymond',   'Santos',       'Perez',     'reymond.santos@demo.com',    '09171234501', '1985-03-15', 'male',   'married',  'Filipino', 1, 1, NULL,  '2022-01-15', '2022-07-15', NULL,          'regular',      'active',   NULL),
    (2,  1, 7,  'EMP-2022-002', 'Carlo',     'Mendoza',      'Reyes',     'carlo.mendoza@demo.com',     '09171234502', '1992-07-22', 'male',   'single',   'Filipino', 1, 2, 1,     '2022-03-01', '2022-09-01', NULL,          'regular',      'active',   NULL),
    (3,  1, 8,  'EMP-2023-003', 'Jasmine',   'Villanueva',   'Cruz',      'jasmine.villanueva@demo.com','09171234503', '1996-11-08', 'female', 'single',   'Filipino', 1, 3, 1,     '2023-06-15', '2023-12-15', NULL,          'regular',      'active',   NULL),
    (4,  1, 9,  'EMP-2024-004', 'Mark',      'Aquino',       NULL,        'mark.aquino@demo.com',       '09171234504', '1998-02-14', 'male',   'single',   'Filipino', 1, 3, 1,     '2024-09-01', NULL,         NULL,          'probationary', 'active',   NULL),
    (5,  1, 10, 'EMP-2023-005', 'Patricia',  'Bautista',     'Santos',    'patricia.bautista@demo.com', '09171234505', '1994-05-30', 'female', 'married',  'Filipino', 1, 4, 1,     '2023-01-10', '2023-07-10', NULL,          'regular',      'active',   NULL),

    -- Human Resources Department
    (6,  1, 3,  'EMP-2022-006', 'Maria',     'Reyes',        'Luna',      'maria.reyes@demo.com',       '09171234506', '1988-09-20', 'female', 'married',  'Filipino', 2, 5, NULL,  '2022-02-01', '2022-08-01', NULL,          'regular',      'active',   NULL),
    (7,  1, 11, 'EMP-2023-007', 'Rafael',    'Dizon',        'Manalo',    'rafael.dizon@demo.com',      '09171234507', '1995-12-03', 'male',   'single',   'Filipino', 2, 6, 6,     '2023-04-01', '2023-10-01', NULL,          'regular',      'active',   NULL),

    -- Finance Department
    (8,  1, 4,  'EMP-2022-008', 'Jose',      'Garcia',       'Tan',       'jose.garcia@demo.com',       '09171234508', '1987-01-25', 'male',   'married',  'Filipino', 3, 7, NULL,  '2022-01-20', '2022-07-20', NULL,          'regular',      'active',   NULL),
    (9,  1, 12, 'EMP-2023-009', 'Grace',     'Soriano',      NULL,        'grace.soriano@demo.com',     '09171234509', '1993-08-17', 'female', 'single',   'Filipino', 3, 8, 8,     '2023-07-01', '2024-01-01', NULL,          'regular',      'active',   NULL),

    -- Marketing Department
    (10, 1, 5,  'EMP-2022-010', 'Ana',       'Cruz',         'Gonzales',  'ana.cruz@demo.com',          '09171234510', '1990-04-12', 'female', 'single',   'Filipino', 4, 9, NULL,  '2022-04-01', '2022-10-01', NULL,          'regular',      'active',   NULL),
    (11, 1, 13, 'EMP-2024-011', 'Kevin',     'Delos Reyes',  NULL,        'kevin.delos-reyes@demo.com', '09171234511', '1997-10-05', 'male',   'single',   'Filipino', 4, 10, 10,   '2024-02-15', '2024-08-15', NULL,          'regular',      'active',   NULL),

    -- Operations Department
    (12, 1, 6,  'EMP-2022-012', 'Miguel',    'Fernandez',    'Lopez',     'miguel.fernandez@demo.com',  '09171234512', '1986-06-28', 'male',   'married',  'Filipino', 5, 11, NULL, '2022-05-01', '2022-11-01', NULL,          'regular',      'active',   NULL),
    (13, 1, 14, 'EMP-2024-013', 'Cristina',  'Lim',          'Ong',       'cristina.lim@demo.com',      '09171234513', '1999-03-19', 'female', 'single',   'Filipino', 5, 12, 12,   '2024-06-01', NULL,         NULL,          'probationary', 'active',   NULL),

    -- Administration
    (14, 1, 15, 'EMP-2023-014', 'Daniel',    'Pascual',      'Rivera',    'daniel.pascual@demo.com',    '09171234514', '1991-11-11', 'male',   'married',  'Filipino', 6, 13, NULL, '2023-03-01', '2023-09-01', NULL,          'regular',      'active',   NULL),

    -- Sales Department
    (15, 1, 16, 'EMP-2025-015', 'Angelica',  'Navarro',      NULL,        'angelica.navarro@demo.com',  '09171234515', '2000-01-07', 'female', 'single',   'Filipino', 7, 14, NULL, '2025-01-15', NULL,         NULL,          'probationary', 'active',   NULL),

    -- Contractual employee
    (16, 1, 17, 'EMP-2025-016', 'Rico',      'Santiago',     'Dela Cruz', 'rico.santiago@demo.com',     '09171234516', '1993-06-14', 'male',   'single',   'Filipino', 1, 3, 1,     '2025-01-02', NULL,         NULL,          'contractual',  'active',   '2025-07-01'),

    -- Resigned employee
    (17, 1, 18, 'EMP-2024-017', 'Diana',     'Ramos',        'Torres',    'diana.ramos@demo.com',       '09171234517', '1994-09-25', 'female', 'married',  'Filipino', 4, 10, 10,   '2024-03-01', '2024-09-01', '2025-12-31', 'regular',      'separated', NULL),

    -- Draft employee (not yet onboarded)
    (18, 1, NULL,'EMP-2026-018', 'Benjamin',  'Ocampo',       NULL,        'benjamin.ocampo@demo.com',   '09171234518', '1997-04-02', 'male',   'single',   'Filipino', 1, 3, 1,     '2026-04-01', NULL,         NULL,          'probationary', 'draft',    NULL)
ON CONFLICT DO NOTHING;

SELECT setval('employees_id_seq', GREATEST((SELECT MAX(id) FROM employees), 18));

-- Set department heads
UPDATE departments SET head_employee_id = 1  WHERE id = 1 AND company_id = 1;
UPDATE departments SET head_employee_id = 6  WHERE id = 2 AND company_id = 1;
UPDATE departments SET head_employee_id = 8  WHERE id = 3 AND company_id = 1;
UPDATE departments SET head_employee_id = 10 WHERE id = 4 AND company_id = 1;
UPDATE departments SET head_employee_id = 12 WHERE id = 5 AND company_id = 1;
UPDATE departments SET head_employee_id = 14 WHERE id = 6 AND company_id = 1;

-- =====================================================================
-- 6. EMPLOYEE PROFILES (sensitive PII data)
-- =====================================================================
INSERT INTO employee_profiles (employee_id, address_line1, city, province, zip_code, emergency_name, emergency_phone, emergency_relation, bank_name, bank_account_no, bank_account_name, tin, sss_no, philhealth_no, pagibig_no, blood_type) VALUES
    (1,  '456 Mabini St, Brgy. Poblacion',     'Makati City',     'Metro Manila', '1200', 'Elena Santos',       '09171111001', 'spouse',  'BDO',          '001234567890', 'Reymond P. Santos',       '123-456-001', '33-1234501-1', '01-234567001-1', '1234-5678-9001', 'O+'),
    (2,  '789 Bonifacio Ave',                   'Taguig City',     'Metro Manila', '1630', 'Rosa Mendoza',       '09171111002', 'mother',  'BPI',          '001234567891', 'Carlo R. Mendoza',        '123-456-002', '33-1234502-2', '01-234567002-2', '1234-5678-9002', 'A+'),
    (3,  '12 Rizal Blvd',                       'Pasig City',      'Metro Manila', '1600', 'Lorna Villanueva',   '09171111003', 'mother',  'Metrobank',    '001234567892', 'Jasmine C. Villanueva',   '123-456-003', '33-1234503-3', '01-234567003-3', '1234-5678-9003', 'B+'),
    (4,  '34 Luna St',                          'Quezon City',     'Metro Manila', '1100', 'Pedro Aquino',       '09171111004', 'father',  'BDO',          '001234567893', 'Mark L. Aquino',          '123-456-004', '33-1234504-4', '01-234567004-4', '1234-5678-9004', 'AB+'),
    (5,  '56 Aguinaldo St',                     'Mandaluyong City','Metro Manila', '1550', 'Roberto Bautista',   '09171111005', 'spouse',  'BPI',          '001234567894', 'Patricia S. Bautista',    '123-456-005', '33-1234505-5', '01-234567005-5', '1234-5678-9005', 'O-'),
    (6,  '78 Del Pilar St',                     'Makati City',     'Metro Manila', '1200', 'Antonio Reyes',      '09171111006', 'spouse',  'Metrobank',    '001234567895', 'Maria L. Reyes',          '123-456-006', '33-1234506-6', '01-234567006-6', '1234-5678-9006', 'A+'),
    (7,  '90 Quezon Ave',                       'Quezon City',     'Metro Manila', '1100', 'Carmen Dizon',       '09171111007', 'mother',  'BDO',          '001234567896', 'Rafael M. Dizon',         '123-456-007', '33-1234507-7', '01-234567007-7', '1234-5678-9007', 'B-'),
    (8,  '23 Osme\u00f1a Blvd',                       'Pasay City',      'Metro Manila', '1300', 'Sofia Garcia',       '09171111008', 'spouse',  'Unionbank',    '001234567897', 'Jose T. Garcia',          '123-456-008', '33-1234508-8', '01-234567008-8', '1234-5678-9008', 'O+'),
    (9,  '45 Roxas Blvd',                       'Manila City',     'Metro Manila', '1000', 'Luz Soriano',        '09171111009', 'mother',  'BPI',          '001234567898', 'Grace L. Soriano',        '123-456-009', '33-1234509-9', '01-234567009-9', '1234-5678-9009', 'A-'),
    (10, '67 Ayala Ave',                        'Makati City',     'Metro Manila', '1200', 'Eduardo Cruz',       '09171111010', 'father',  'Metrobank',    '001234567899', 'Ana G. Cruz',             '123-456-010', '33-1234510-0', '01-234567010-0', '1234-5678-9010', 'AB-'),
    (11, '89 Katipunan Ave',                    'Quezon City',     'Metro Manila', '1100', 'Dolores Delos Reyes','09171111011', 'mother',  'BDO',          '001234567900', 'Kevin L. Delos Reyes',    '123-456-011', '33-1234511-1', '01-234567011-1', '1234-5678-9011', 'O+'),
    (12, '11 Taft Ave',                         'Pasay City',      'Metro Manila', '1300', 'Carmen Fernandez',   '09171111012', 'spouse',  'BPI',          '001234567901', 'Miguel L. Fernandez',     '123-456-012', '33-1234512-2', '01-234567012-2', '1234-5678-9012', 'B+'),
    (13, '33 Shaw Blvd',                        'Mandaluyong City','Metro Manila', '1550', 'Ricardo Lim',        '09171111013', 'father',  'Metrobank',    '001234567902', 'Cristina O. Lim',         '123-456-013', '33-1234513-3', '01-234567013-3', '1234-5678-9013', 'A+'),
    (14, '55 Ortigas Ave',                      'Pasig City',      'Metro Manila', '1600', 'Rosa Pascual',       '09171111014', 'mother',  'BDO',          '001234567903', 'Daniel R. Pascual',       '123-456-014', '33-1234514-4', '01-234567014-4', '1234-5678-9014', 'O+'),
    (15, '77 Jupiter St',                       'Makati City',     'Metro Manila', '1200', 'Maricel Navarro',    '09171111015', 'mother',  'BPI',          '001234567904', 'Angelica L. Navarro',     '123-456-015', '33-1234515-5', '01-234567015-5', '1234-5678-9015', 'AB+'),
    (16, '99 Commonwealth Ave',                 'Quezon City',     'Metro Manila', '1100', 'Manuel Santiago',    '09171111016', 'father',  'Unionbank',    '001234567905', 'Rico D. Santiago',        '123-456-016', '33-1234516-6', '01-234567016-6', '1234-5678-9016', 'B-'),
    (17, '22 España Blvd',                      'Manila City',     'Metro Manila', '1000', 'Carlos Ramos',       '09171111017', 'spouse',  'Metrobank',    '001234567906', 'Diana T. Ramos',          '123-456-017', '33-1234517-7', '01-234567017-7', '1234-5678-9017', 'A+')
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 7. SHIFTS & SCHEDULE TEMPLATES
-- =====================================================================
INSERT INTO shifts (id, company_id, name, start_time, end_time, break_minutes, grace_minutes, is_overnight, is_active) VALUES
    (1, 1, 'Regular Day Shift',   '08:00:00', '17:00:00', 60, 15, false, true),
    (2, 1, 'Morning Shift',       '06:00:00', '15:00:00', 60, 15, false, true),
    (3, 1, 'Mid Shift',           '10:00:00', '19:00:00', 60, 15, false, true),
    (4, 1, 'Night Shift',         '22:00:00', '07:00:00', 60, 15, true,  true)
ON CONFLICT DO NOTHING;

SELECT setval('shifts_id_seq', GREATEST((SELECT MAX(id) FROM shifts), 4));

-- Schedule template: Regular Mon-Fri
INSERT INTO schedule_templates (id, company_id, name, description, is_active) VALUES
    (1, 1, 'Regular Mon-Fri (8AM-5PM)', 'Standard office hours, Mon-Fri with Sat-Sun rest days', true),
    (2, 1, 'Mid Shift Mon-Fri',         '10AM-7PM shift, Mon-Fri',                               true)
ON CONFLICT DO NOTHING;

SELECT setval('schedule_templates_id_seq', GREATEST((SELECT MAX(id) FROM schedule_templates), 2));

-- Template 1: Mon-Fri day shift, Sat-Sun rest
INSERT INTO schedule_template_days (template_id, day_of_week, shift_id, is_rest_day) VALUES
    (1, 0, NULL, true),   -- Sunday rest
    (1, 1, 1,    false),  -- Monday
    (1, 2, 1,    false),  -- Tuesday
    (1, 3, 1,    false),  -- Wednesday
    (1, 4, 1,    false),  -- Thursday
    (1, 5, 1,    false),  -- Friday
    (1, 6, NULL, true)    -- Saturday rest
ON CONFLICT DO NOTHING;

-- Template 2: Mon-Fri mid shift
INSERT INTO schedule_template_days (template_id, day_of_week, shift_id, is_rest_day) VALUES
    (2, 0, NULL, true),
    (2, 1, 3,    false),
    (2, 2, 3,    false),
    (2, 3, 3,    false),
    (2, 4, 3,    false),
    (2, 5, 3,    false),
    (2, 6, NULL, true)
ON CONFLICT DO NOTHING;

-- Assign all active employees to regular schedule
INSERT INTO employee_schedule_assignments (company_id, employee_id, template_id, effective_from)
SELECT 1, e.id, 1, e.hire_date
FROM employees e
WHERE e.company_id = 1 AND e.status IN ('active', 'draft') AND e.id <= 18
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 8. LEAVE TYPES (Philippine standard)
-- =====================================================================
INSERT INTO leave_types (id, company_id, code, name, is_paid, default_days, is_convertible, requires_attachment, min_days_notice, accrual_type, gender_specific, is_statutory, is_active) VALUES
    (1, 1, 'VL',       'Vacation Leave',           true,  15,   true,  false, 5,  'annual',  NULL,     false, true),
    (2, 1, 'SL',       'Sick Leave',               true,  15,   false, true,  0,  'annual',  NULL,     false, true),
    (3, 1, 'SIL',      'Service Incentive Leave',  true,  5,    true,  false, 0,  'annual',  NULL,     true,  true),
    (4, 1, 'ML',       'Maternity Leave',          true,  105,  false, true,  30, 'none',    'female', true,  true),
    (5, 1, 'PL',       'Paternity Leave',          true,  7,    false, true,  0,  'none',    'male',   true,  true),
    (6, 1, 'SPL',      'Solo Parent Leave',        true,  7,    false, true,  0,  'annual',  NULL,     true,  true),
    (7, 1, 'VAWC',     'VAWC Leave',               true,  10,   false, true,  0,  'none',    'female', true,  true),
    (8, 1, 'EL',       'Emergency Leave',          true,  3,    false, false, 0,  'annual',  NULL,     false, true),
    (9, 1, 'BL',       'Bereavement Leave',        true,  5,    false, true,  0,  'none',    NULL,     false, true),
    (10,1, 'LWOP',     'Leave Without Pay',        false, 0,    false, false, 3,  'none',    NULL,     false, true)
ON CONFLICT DO NOTHING;

SELECT setval('leave_types_id_seq', GREATEST((SELECT MAX(id) FROM leave_types), 10));

-- =====================================================================
-- 9. LEAVE BALANCES (2026, for active employees)
-- =====================================================================
INSERT INTO leave_balances (company_id, employee_id, leave_type_id, year, earned, used, carried, adjusted)
SELECT 1, e.id, lt.id, 2026,
    CASE
        WHEN lt.code = 'VL' THEN 15
        WHEN lt.code = 'SL' THEN 15
        WHEN lt.code = 'SIL' THEN 5
        WHEN lt.code = 'EL' THEN 3
        WHEN lt.code = 'BL' THEN 5
        ELSE 0
    END,
    CASE
        WHEN lt.code = 'VL' AND e.id IN (1, 2, 5, 10) THEN 3
        WHEN lt.code = 'SL' AND e.id IN (3, 7, 9) THEN 2
        ELSE 0
    END,
    CASE
        WHEN lt.code = 'VL' AND e.hire_date < '2025-01-01' THEN 2
        ELSE 0
    END,
    0
FROM employees e
CROSS JOIN leave_types lt
WHERE e.company_id = 1
  AND e.status = 'active'
  AND lt.company_id = 1
  AND lt.code IN ('VL', 'SL', 'SIL', 'EL', 'BL')
  AND lt.is_active = true
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 10. LEAVE REQUESTS (various statuses)
-- =====================================================================
INSERT INTO leave_requests (id, company_id, employee_id, leave_type_id, start_date, end_date, days, reason, status, approver_id, approved_at, rejection_reason) VALUES
    -- Approved vacation
    (1, 1, 2,  1, '2026-03-16', '2026-03-18', 3, 'Family vacation to Boracay', 'approved', 1, '2026-02-20 10:00:00+08', NULL),
    -- Pending sick leave
    (2, 1, 3,  2, '2026-03-10', '2026-03-11', 2, 'Flu and fever', 'pending', NULL, NULL, NULL),
    -- Approved SIL
    (3, 1, 7,  3, '2026-03-05', '2026-03-05', 1, 'Personal errand', 'approved', 6, '2026-03-03 14:30:00+08', NULL),
    -- Rejected leave
    (4, 1, 11, 1, '2026-03-25', '2026-03-28', 4, 'Planned trip', 'rejected', 10, '2026-03-10 09:00:00+08', 'Critical project deadline during this period'),
    -- Pending emergency leave
    (5, 1, 9,  8, '2026-03-07', '2026-03-07', 1, 'Pipe burst at home, need emergency repair', 'pending', NULL, NULL, NULL),
    -- Cancelled leave
    (6, 1, 5,  1, '2026-04-01', '2026-04-03', 3, 'Vacation plans', 'cancelled', NULL, NULL, NULL)
ON CONFLICT DO NOTHING;

SELECT setval('leave_requests_id_seq', GREATEST((SELECT MAX(id) FROM leave_requests), 6));

-- =====================================================================
-- 11. ATTENDANCE LOGS (last 7 days for active employees)
-- Generate clock in/out for 2026-02-27 through 2026-03-05 (Mon-Fri only)
-- Using CURRENT_DATE - interval for relative dates
-- =====================================================================

-- +goose StatementBegin
DO $$
DECLARE
    emp_record RECORD;
    work_day DATE;
    day_counter INT;
    clock_in_offset INT;  -- minutes offset from 08:00
    clock_out_offset INT; -- minutes offset from 17:00
    work_hrs NUMERIC(5,2);
    late_mins INT;
    ut_mins INT;
    att_status VARCHAR(20);
BEGIN
    -- Generate attendance for past 7 working days
    FOR day_counter IN 0..9 LOOP
        work_day := CURRENT_DATE - day_counter;

        -- Skip weekends
        CONTINUE WHEN EXTRACT(DOW FROM work_day) IN (0, 6);

        FOR emp_record IN
            SELECT id FROM employees
            WHERE company_id = 1 AND status = 'active' AND id <= 17
        LOOP
            -- Skip if record already exists for this day
            IF EXISTS (
                SELECT 1 FROM attendance_logs
                WHERE employee_id = emp_record.id
                  AND clock_in_at::date = work_day
            ) THEN
                CONTINUE;
            END IF;

            -- Vary arrival times: most on time, some late
            CASE
                WHEN emp_record.id IN (4, 11, 16) THEN
                    -- These employees tend to be late (15-45 min)
                    clock_in_offset := 15 + (emp_record.id * day_counter) % 30;
                WHEN emp_record.id IN (1, 6, 8, 12) THEN
                    -- Managers arrive early (-15 to 0 min)
                    clock_in_offset := -15 + (emp_record.id + day_counter) % 15;
                ELSE
                    -- Others: -5 to +10 min
                    clock_in_offset := -5 + (emp_record.id + day_counter) % 15;
            END CASE;

            -- Vary departure times
            CASE
                WHEN emp_record.id IN (1, 2, 5) THEN
                    -- Sometimes work overtime (0-60 min after 17:00)
                    clock_out_offset := (emp_record.id + day_counter) % 60;
                WHEN emp_record.id IN (13, 15) THEN
                    -- Sometimes leave early (-30 to 0 min)
                    clock_out_offset := -30 + (emp_record.id + day_counter) % 30;
                ELSE
                    clock_out_offset := -5 + (emp_record.id + day_counter) % 15;
            END CASE;

            -- Calculate late minutes (only if arrived after 08:15 grace period)
            late_mins := GREATEST(0, clock_in_offset - 15);

            -- Calculate undertime (only if left before 17:00)
            ut_mins := GREATEST(0, -clock_out_offset);

            -- Calculate work hours
            work_hrs := ROUND(((9 * 60 - 60 - GREATEST(0, clock_in_offset) + GREATEST(0, clock_out_offset))::NUMERIC / 60), 2);
            work_hrs := GREATEST(work_hrs, 0);

            -- Determine status
            IF late_mins > 0 THEN
                att_status := 'late';
            ELSE
                att_status := 'present';
            END IF;

            INSERT INTO attendance_logs (
                company_id, employee_id, clock_in_at, clock_out_at,
                clock_in_source, clock_out_source,
                clock_in_lat, clock_in_lng, clock_out_lat, clock_out_lng,
                work_hours, overtime_hours, late_minutes, undertime_minutes,
                status
            ) VALUES (
                1, emp_record.id,
                (work_day + '08:00:00'::time + (clock_in_offset || ' minutes')::interval) AT TIME ZONE 'Asia/Manila',
                (work_day + '17:00:00'::time + (clock_out_offset || ' minutes')::interval) AT TIME ZONE 'Asia/Manila',
                'web', 'web',
                14.5547 + (random() * 0.001), 121.0244 + (random() * 0.001),  -- Makati area coords
                14.5547 + (random() * 0.001), 121.0244 + (random() * 0.001),
                work_hrs,
                CASE WHEN clock_out_offset > 0 THEN ROUND(clock_out_offset::NUMERIC / 60, 2) ELSE 0 END,
                late_mins,
                ut_mins,
                att_status
            );
        END LOOP;
    END LOOP;
END;
$$;
-- +goose StatementEnd

-- =====================================================================
-- 12. OVERTIME REQUESTS
-- =====================================================================
INSERT INTO overtime_requests (id, company_id, employee_id, ot_date, start_at, end_at, hours, ot_type, reason, status, approver_id, approved_at) VALUES
    (1, 1, 2,  '2026-03-03', '2026-03-03 17:00:00+08', '2026-03-03 20:00:00+08', 3, 'regular',  'Sprint deadline - API integration completion', 'approved', 1, '2026-03-02 16:00:00+08'),
    (2, 1, 5,  '2026-03-04', '2026-03-04 17:00:00+08', '2026-03-04 19:00:00+08', 2, 'regular',  'QA regression testing for release v2.1',       'pending',  NULL, NULL),
    (3, 1, 11, '2026-03-01', '2026-03-01 09:00:00+08', '2026-03-01 15:00:00+08', 6, 'rest_day', 'Marketing campaign launch preparation',        'approved', 10, '2026-02-28 11:00:00+08')
ON CONFLICT DO NOTHING;

SELECT setval('overtime_requests_id_seq', GREATEST((SELECT MAX(id) FROM overtime_requests), 3));

-- =====================================================================
-- 13. SALARY STRUCTURES & COMPONENTS
-- =====================================================================
INSERT INTO salary_structures (id, company_id, name, description, is_active) VALUES
    (1, 1, 'Standard Structure',  'Default salary structure for regular employees', true),
    (2, 1, 'Executive Structure', 'Structure for managerial and executive roles',   true)
ON CONFLICT DO NOTHING;

SELECT setval('salary_structures_id_seq', GREATEST((SELECT MAX(id) FROM salary_structures), 2));

INSERT INTO salary_components (id, company_id, code, name, component_type, is_taxable, is_statutory, is_fixed, formula, is_active) VALUES
    (1, 1, 'BASIC',          'Basic Salary',              'earning',    true,  false, true,  NULL, true),
    (2, 1, 'RICE',           'Rice Allowance',            'earning',    false, false, true,  NULL, true),
    (3, 1, 'TRANSPORT',      'Transportation Allowance',  'earning',    false, false, true,  NULL, true),
    (4, 1, 'CLOTHING',       'Clothing Allowance',        'earning',    false, false, true,  NULL, true),
    (5, 1, 'SSS_EE',         'SSS Employee Share',        'deduction',  false, true,  false, '{"type":"table","table":"sss"}'::jsonb, true),
    (6, 1, 'PHILHEALTH_EE',  'PhilHealth Employee Share', 'deduction',  false, true,  false, '{"type":"table","table":"philhealth"}'::jsonb, true),
    (7, 1, 'PAGIBIG_EE',     'Pag-IBIG Employee Share',   'deduction',  false, true,  false, '{"type":"table","table":"pagibig"}'::jsonb, true),
    (8, 1, 'TAX',            'Withholding Tax',           'deduction',  false, true,  false, '{"type":"table","table":"bir"}'::jsonb, true)
ON CONFLICT DO NOTHING;

SELECT setval('salary_components_id_seq', GREATEST((SELECT MAX(id) FROM salary_components), 8));

-- =====================================================================
-- 14. EMPLOYEE SALARIES (monthly basic salary)
-- =====================================================================
INSERT INTO employee_salaries (id, company_id, employee_id, structure_id, basic_salary, effective_from, remarks, created_by) VALUES
    (1,  1, 1,  2, 85000.00, '2022-01-15', 'Initial salary - CTO',                    1),
    (2,  1, 2,  1, 55000.00, '2022-03-01', 'Initial salary - Senior Developer',        1),
    (3,  1, 3,  1, 35000.00, '2023-06-15', 'Initial salary - Junior Developer',        1),
    (4,  1, 4,  1, 28000.00, '2024-09-01', 'Initial salary - Junior Developer (probi)', 1),
    (5,  1, 5,  1, 50000.00, '2023-01-10', 'Initial salary - QA Lead',                 1),
    (6,  1, 6,  2, 65000.00, '2022-02-01', 'Initial salary - HR Manager',              1),
    (7,  1, 7,  1, 32000.00, '2023-04-01', 'Initial salary - HR Specialist',           1),
    (8,  1, 8,  2, 70000.00, '2022-01-20', 'Initial salary - Finance Manager',         1),
    (9,  1, 9,  1, 38000.00, '2023-07-01', 'Initial salary - Accountant',              1),
    (10, 1, 10, 2, 60000.00, '2022-04-01', 'Initial salary - Marketing Manager',       1),
    (11, 1, 11, 1, 30000.00, '2024-02-15', 'Initial salary - Marketing Associate',     1),
    (12, 1, 12, 2, 65000.00, '2022-05-01', 'Initial salary - Operations Manager',      1),
    (13, 1, 13, 1, 25000.00, '2024-06-01', 'Initial salary - Ops Coordinator (probi)', 1),
    (14, 1, 14, 1, 28000.00, '2023-03-01', 'Initial salary - Admin Assistant',         1),
    (15, 1, 15, 1, 22000.00, '2025-01-15', 'Initial salary - Sales Rep (probi)',       1),
    (16, 1, 16, 1, 30000.00, '2025-01-02', 'Initial salary - Contractual Developer',   1),
    (17, 1, 17, 1, 30000.00, '2024-03-01', 'Initial salary - Marketing Associate',     1)
ON CONFLICT DO NOTHING;

SELECT setval('employee_salaries_id_seq', GREATEST((SELECT MAX(id) FROM employee_salaries), 17));

-- =====================================================================
-- 15. PAYROLL CYCLES & RUNS (Jan 2026, Feb 2026)
-- =====================================================================
INSERT INTO payroll_cycles (id, company_id, name, period_start, period_end, pay_date, cycle_type, status, created_by) VALUES
    -- January 2026
    (1, 1, 'January 2026 - 1st Half', '2026-01-01', '2026-01-15', '2026-01-15', 'regular', 'paid', 1),
    (2, 1, 'January 2026 - 2nd Half', '2026-01-16', '2026-01-31', '2026-01-31', 'regular', 'paid', 1),
    -- February 2026
    (3, 1, 'February 2026 - 1st Half', '2026-02-01', '2026-02-15', '2026-02-15', 'regular', 'paid', 1),
    (4, 1, 'February 2026 - 2nd Half', '2026-02-16', '2026-02-28', '2026-02-28', 'regular', 'paid', 1),
    -- March 2026 (current, in progress)
    (5, 1, 'March 2026 - 1st Half', '2026-03-01', '2026-03-15', '2026-03-15', 'regular', 'draft', 1)
ON CONFLICT DO NOTHING;

SELECT setval('payroll_cycles_id_seq', GREATEST((SELECT MAX(id) FROM payroll_cycles), 5));

-- Payroll runs for completed cycles
INSERT INTO payroll_runs (id, company_id, cycle_id, run_type, run_number, total_employees, total_gross, total_deductions, total_net, status, initiated_by, completed_at) VALUES
    (1, 1, 1, 'regular', 1, 16, 450000.00, 95000.00, 355000.00, 'completed', 1, '2026-01-15 08:00:00+08'),
    (2, 1, 2, 'regular', 1, 16, 450000.00, 95000.00, 355000.00, 'completed', 1, '2026-01-31 08:00:00+08'),
    (3, 1, 3, 'regular', 1, 16, 450000.00, 95000.00, 355000.00, 'completed', 1, '2026-02-15 08:00:00+08'),
    (4, 1, 4, 'regular', 1, 16, 450000.00, 95000.00, 355000.00, 'completed', 1, '2026-02-28 08:00:00+08')
ON CONFLICT DO NOTHING;

SELECT setval('payroll_runs_id_seq', GREATEST((SELECT MAX(id) FROM payroll_runs), 4));

-- Sample payroll items for the latest completed run (Feb 2nd half - run_id=4)
INSERT INTO payroll_items (run_id, employee_id, basic_pay, gross_pay, taxable_income, total_deductions, net_pay, sss_ee, sss_er, sss_ec, philhealth_ee, philhealth_er, pagibig_ee, pagibig_er, withholding_tax, breakdown, work_days, hours_worked) VALUES
    (4, 1,  42500, 44000, 41300, 7595.00, 36405.00, 1125.00, 2375.00, 30, 2125.00, 2125.00, 100, 100, 4245.00, '{"rice_allowance":1500}', 10, 80),
    (4, 2,  27500, 29000, 26300, 4430.00, 24570.00, 900.00,  1900.00, 30, 1375.00, 1375.00, 100, 100, 2055.00, '{"rice_allowance":1500}', 10, 80),
    (4, 3,  17500, 19000, 16300, 2525.00, 16475.00, 675.00,  1425.00, 30, 875.00,  875.00,  100, 100, 875.00,  '{"rice_allowance":1500}', 10, 80),
    (4, 4,  14000, 15500, 12800, 1905.00, 13595.00, 540.00,  1140.00, 10, 700.00,  700.00,  100, 100, 565.00,  '{"rice_allowance":1500}', 10, 80),
    (4, 5,  25000, 26500, 23800, 3805.00, 22695.00, 900.00,  1900.00, 30, 1250.00, 1250.00, 100, 100, 1555.00, '{"rice_allowance":1500}', 10, 80),
    (4, 6,  32500, 34000, 31300, 5705.00, 28295.00, 1080.00, 2280.00, 30, 1625.00, 1625.00, 100, 100, 2900.00, '{"rice_allowance":1500}', 10, 80),
    (4, 7,  16000, 17500, 14800, 2255.00, 15245.00, 630.00,  1330.00, 10, 800.00,  800.00,  100, 100, 725.00,  '{"rice_allowance":1500}', 10, 80),
    (4, 8,  35000, 36500, 33800, 6205.00, 30295.00, 1125.00, 2375.00, 30, 1750.00, 1750.00, 100, 100, 3230.00, '{"rice_allowance":1500}', 10, 80),
    (4, 9,  19000, 20500, 17800, 2775.00, 17725.00, 720.00,  1520.00, 30, 950.00,  950.00,  100, 100, 1005.00, '{"rice_allowance":1500}', 10, 80),
    (4, 10, 30000, 31500, 28800, 5130.00, 26370.00, 1012.50, 2137.50, 30, 1500.00, 1500.00, 100, 100, 2517.50, '{"rice_allowance":1500}', 10, 80),
    (4, 11, 15000, 16500, 13800, 2105.00, 14395.00, 585.00,  1235.00, 10, 750.00,  750.00,  100, 100, 670.00,  '{"rice_allowance":1500}', 10, 80),
    (4, 12, 32500, 34000, 31300, 5705.00, 28295.00, 1080.00, 2280.00, 30, 1625.00, 1625.00, 100, 100, 2900.00, '{"rice_allowance":1500}', 10, 80),
    (4, 13, 12500, 14000, 11300, 1665.00, 12335.00, 450.00,  950.00,  10, 625.00,  625.00,  100, 100, 490.00,  '{"rice_allowance":1500}', 10, 80),
    (4, 14, 14000, 15500, 12800, 1905.00, 13595.00, 540.00,  1140.00, 10, 700.00,  700.00,  100, 100, 565.00,  '{"rice_allowance":1500}', 10, 80),
    (4, 15, 11000, 12500, 9800,  1380.00, 11120.00, 405.00,  855.00,  10, 550.00,  550.00,  100, 100, 325.00,  '{"rice_allowance":1500}', 10, 80),
    (4, 16, 15000, 16500, 13800, 2105.00, 14395.00, 585.00,  1235.00, 10, 750.00,  750.00,  100, 100, 670.00,  '{"rice_allowance":1500}', 10, 80)
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 16. LOAN TYPES & LOANS
-- =====================================================================
INSERT INTO loan_types (id, company_id, name, code, provider, max_term_months, interest_rate, max_amount, requires_approval, auto_deduct, is_active) VALUES
    (1, 1, 'SSS Salary Loan',         'sss_salary',    'government', 24, 0.01,   50000,  true, true, true),
    (2, 1, 'Pag-IBIG Multi-Purpose',   'pagibig_mpl',   'government', 24, 0.0087, 80000,  true, true, true),
    (3, 1, 'Company Cash Advance',     'cash_advance',  'company',    6,  0,      20000,  true, true, true),
    (4, 1, 'Pag-IBIG Calamity Loan',   'pagibig_cal',   'government', 24, 0.0087, 50000,  true, true, true),
    (5, 1, 'SSS Calamity Loan',        'sss_calamity',  'government', 24, 0.01,   30000,  true, true, true)
ON CONFLICT DO NOTHING;

SELECT setval('loan_types_id_seq', GREATEST((SELECT MAX(id) FROM loan_types), 5));

-- Active loans
INSERT INTO loans (id, company_id, employee_id, loan_type_id, reference_no, principal_amount, interest_rate, term_months, monthly_amortization, total_amount, total_paid, remaining_balance, start_date, end_date, status, approved_by, approved_at, remarks) VALUES
    (1, 1, 2,  1, 'SSS-2025-001234', 50000, 0.01, 24, 2333.33, 56000, 9333.32, 46666.68, '2025-07-01', '2027-06-30', 'active', 1, '2025-06-25 10:00:00+08', 'SSS salary loan - 2 year term'),
    (2, 1, 9,  2, 'PAGIBIG-2025-005678', 40000, 0.0087, 24, 1847.00, 44328, 5541.00, 38787.00, '2025-10-01', '2027-09-30', 'active', 8, '2025-09-20 14:00:00+08', 'Pag-IBIG multi-purpose loan'),
    (3, 1, 14, 3, NULL, 15000, 0, 6, 2500, 15000, 5000, 10000, '2025-12-01', '2026-05-31', 'active', 1, '2025-11-28 09:00:00+08', 'Emergency cash advance for medical expenses'),
    (4, 1, 11, 3, NULL, 10000, 0, 3, 3333.33, 10000, 0, 10000, '2026-03-01', '2026-05-31', 'pending', NULL, NULL, 'Cash advance request for family emergency')
ON CONFLICT DO NOTHING;

SELECT setval('loans_id_seq', GREATEST((SELECT MAX(id) FROM loans), 4));

-- Loan payments
INSERT INTO loan_payments (loan_id, payment_date, amount, principal_portion, interest_portion, payment_type, remarks) VALUES
    (1, '2025-07-31', 2333.33, 1833.33, 500.00, 'payroll', 'July 2025 deduction'),
    (1, '2025-08-31', 2333.33, 1851.67, 481.66, 'payroll', 'August 2025 deduction'),
    (1, '2025-09-30', 2333.33, 1870.18, 463.15, 'payroll', 'September 2025 deduction'),
    (1, '2025-10-31', 2333.33, 1888.89, 444.44, 'payroll', 'October 2025 deduction'),
    (2, '2025-10-31', 1847.00, 1499.00, 348.00, 'payroll', 'October 2025 deduction'),
    (2, '2025-11-30', 1847.00, 1512.00, 335.00, 'payroll', 'November 2025 deduction'),
    (2, '2025-12-31', 1847.00, 1525.00, 322.00, 'payroll', 'December 2025 deduction'),
    (3, '2025-12-31', 2500.00, 2500.00, 0,       'payroll', 'December 2025 deduction'),
    (3, '2026-01-31', 2500.00, 2500.00, 0,       'payroll', 'January 2026 deduction')
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 17. ANNOUNCEMENTS
-- =====================================================================
INSERT INTO announcements (id, company_id, title, content, priority, target_roles, target_departments, published_at, expires_at, created_by) VALUES
    (1, 1,
     'Q1 2026 Town Hall Meeting',
     'Dear Team,\n\nPlease join us for our quarterly town hall meeting on March 15, 2026 at 2:00 PM in the main conference room. We will discuss Q1 results, upcoming projects, and company updates.\n\nAttendance is mandatory for all employees.\n\nRefreshments will be provided.\n\nBest regards,\nHR Department',
     'high', NULL, NULL,
     '2026-03-01 09:00:00+08', '2026-03-15 23:59:59+08', 1),
    (2, 1,
     'Updated Leave Policy Effective March 2026',
     'Please be informed that the company has updated its leave policy. Key changes:\n\n1. Vacation leave can now be filed up to 3 days in advance (previously 5 days)\n2. Emergency leave increased from 3 to 5 days per year\n3. Mental health days now covered under sick leave\n\nPlease review the full policy in the HR portal.\n\nFor questions, contact HR at hr@demo.com.',
     'normal', NULL, NULL,
     '2026-02-25 10:00:00+08', '2026-04-30 23:59:59+08', 1),
    (3, 1,
     'IT Maintenance: System Downtime on March 8',
     'The HRIS system will undergo scheduled maintenance on Saturday, March 8, 2026 from 10:00 PM to 2:00 AM. During this time, the system will be unavailable.\n\nPlease plan your attendance logging accordingly. You may use manual time-in sheets which will be corrected on Monday.\n\nThank you for your understanding.',
     'urgent', '{admin,manager}', NULL,
     '2026-03-05 14:00:00+08', '2026-03-09 23:59:59+08', 1)
ON CONFLICT DO NOTHING;

SELECT setval('announcements_id_seq', GREATEST((SELECT MAX(id) FROM announcements), 3));

-- =====================================================================
-- 18. KNOWLEDGE BASE ARTICLES (company-specific)
-- =====================================================================
INSERT INTO knowledge_articles (company_id, category, topic, title, content, tags, source) VALUES
    (1, 'hr_policy', 'work_from_home', 'Work From Home Policy',
     'Demo Company allows employees to work from home up to 2 days per week (Tuesday and Thursday). Requirements: 1) Must have completed probationary period. 2) Must have manager approval. 3) Must be reachable via company communication channels during work hours. 4) Must clock in/out via the HRIS mobile app with GPS verification. WFH privileges may be revoked for performance issues.',
     '{wfh,remote,work_from_home,flexible}', 'Company Policy Manual v3.0'),

    (1, 'hr_policy', 'dress_code', 'Office Dress Code Policy',
     'Business casual attire is required Monday through Thursday. Smart casual is allowed on Fridays (no flip-flops, tank tops, or shorts). Client-facing meetings require business formal attire. Engineering team may wear company-provided t-shirts on non-meeting days. Violations will receive a verbal reminder for the first offense and a written memo for subsequent offenses.',
     '{dress_code,attire,uniform,policy}', 'Company Policy Manual v3.0'),

    (1, 'benefits', 'hmo_coverage', 'HMO Benefits Guide',
     'All regular employees are entitled to HMO coverage through PhilCare after completing 3 months of service. Coverage includes: 1) Up to PHP 200,000 annual benefit limit. 2) 1 dependent (spouse or child). 3) Outpatient consultations, lab tests, and dental. 4) Emergency room coverage. To file a claim, submit the completed HMO form and receipts to HR within 30 days of the medical service. Pre-existing conditions have a 12-month waiting period.',
     '{hmo,health,medical,insurance,philcare}', 'Benefits Handbook 2026'),

    (1, 'payroll', 'payroll_schedule', 'Payroll Schedule and Cutoffs',
     'Demo Company follows a semi-monthly payroll schedule. 1st cutoff: 1st-15th of the month, paid on the 15th. 2nd cutoff: 16th-end of month, paid on the last working day. Overtime and attendance corrections must be submitted 3 days before cutoff. Late submissions will be processed in the next payroll cycle. Government contributions (SSS, PhilHealth, Pag-IBIG) are deducted monthly on the 2nd cutoff.',
     '{payroll,salary,cutoff,schedule,payment}', 'Payroll SOP Document'),

    (1, 'leave', 'leave_filing_guide', 'How to File Leave Requests',
     'Step-by-step guide for filing leave: 1) Log in to the HRIS portal. 2) Navigate to Leave > New Request. 3) Select leave type and dates. 4) Provide reason (required for sick leave and emergency leave). 5) Attach supporting documents if required (medical certificate for 2+ sick days). 6) Submit for approval. Your direct manager will be notified automatically. Approved leave will be reflected in your balance. Cancellation must be done at least 1 day before the leave start date.',
     '{leave,filing,request,howto,guide}', 'HRIS User Guide v2.0')
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 19. PERFORMANCE REVIEW CYCLES & REVIEWS
-- =====================================================================
INSERT INTO review_cycles (id, company_id, name, cycle_type, period_start, period_end, review_deadline, status, created_by) VALUES
    (1, 1, '2025 Annual Performance Review',  'annual',    '2025-01-01', '2025-12-31', '2026-01-31', 'closed',  1),
    (2, 1, '2026 Q1 Performance Review',       'quarterly', '2026-01-01', '2026-03-31', '2026-04-15', 'active',  1)
ON CONFLICT DO NOTHING;

SELECT setval('review_cycles_id_seq', GREATEST((SELECT MAX(id) FROM review_cycles), 2));

-- 2025 annual reviews (completed)
INSERT INTO performance_reviews (company_id, review_cycle_id, employee_id, reviewer_id, status, self_rating, self_comments, self_submitted_at, manager_rating, manager_comments, manager_submitted_at, final_rating, strengths, improvements) VALUES
    (1, 1, 2,  1,  'completed', 4, 'Delivered all sprint commitments on time. Led migration to new framework.',                '2026-01-10 10:00:00+08', 4, 'Excellent technical skills. Good team player.',                   '2026-01-15 10:00:00+08', 4, 'Technical leadership, mentoring', 'Documentation, time estimation'),
    (1, 1, 3,  1,  'completed', 3, 'Improved my coding skills significantly. Completed 3 training courses.',                  '2026-01-08 14:00:00+08', 4, 'Great growth trajectory. Very proactive learner.',                '2026-01-16 09:00:00+08', 4, 'Fast learner, initiative',        'Code review participation'),
    (1, 1, 5,  1,  'completed', 4, 'Reduced bug escape rate by 30%. Implemented automated testing pipeline.',                 '2026-01-09 11:00:00+08', 5, 'Outstanding contribution to quality improvement.',                '2026-01-14 16:00:00+08', 5, 'Quality focus, automation',       'Cross-team communication'),
    (1, 1, 7,  6,  'completed', 3, 'Successfully handled onboarding for 5 new employees. Updated all HR SOPs.',               '2026-01-12 09:00:00+08', 3, 'Solid performance. Handles routine tasks well.',                  '2026-01-17 11:00:00+08', 3, 'Organization, attention to detail','Strategic thinking, initiative'),
    (1, 1, 9,  8,  'completed', 4, 'Streamlined monthly closing process. Reduced closing time from 5 to 3 days.',             '2026-01-11 15:00:00+08', 4, 'Excellent analytical skills. Very reliable.',                     '2026-01-18 10:00:00+08', 4, 'Analytical skills, reliability',  'Presentation skills'),
    (1, 1, 11, 10, 'completed', 3, 'Managed social media campaigns with 20% engagement increase.',                            '2026-01-13 10:00:00+08', 3, 'Good creativity. Needs more strategic thinking.',                 '2026-01-19 14:00:00+08', 3, 'Creativity, content creation',    'Analytics, strategic planning')
ON CONFLICT DO NOTHING;

-- 2026 Q1 reviews (in progress)
INSERT INTO performance_reviews (company_id, review_cycle_id, employee_id, reviewer_id, status, self_rating, self_comments, self_submitted_at) VALUES
    (1, 2, 2,  1,  'self_review', 4, 'Leading the new microservices architecture project. On track for Q1 deliverables.', '2026-03-02 10:00:00+08'),
    (1, 2, 3,  1,  'pending', NULL, NULL, NULL),
    (1, 2, 5,  1,  'self_review', 4, 'Implemented end-to-end test automation for the payroll module.', '2026-03-04 15:00:00+08'),
    (1, 2, 7,  6,  'pending', NULL, NULL, NULL),
    (1, 2, 9,  8,  'self_review', 3, 'Working on automating tax filing reports.', '2026-03-03 11:00:00+08')
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 20. GOALS
-- =====================================================================
INSERT INTO goals (company_id, employee_id, review_cycle_id, title, description, category, weight, target_value, actual_value, status, due_date) VALUES
    (1, 2, 2, 'Complete API v2 Migration',       'Migrate all API endpoints to v2 architecture',           'individual', 40.00, '100% endpoints migrated', '65% complete', 'active', '2026-03-31'),
    (1, 2, 2, 'Mentor 2 Junior Developers',      'Conduct weekly 1-on-1 sessions with junior devs',       'team',       30.00, '24 sessions completed',   '18 sessions',  'active', '2026-03-31'),
    (1, 5, 2, 'Achieve 90% Test Coverage',        'Increase automated test coverage for core modules',     'individual', 50.00, '90% coverage',            '82% coverage', 'active', '2026-03-31'),
    (1, 9, 2, 'Automate Monthly Tax Reports',     'Build automated generation for BIR 1601-C and 2316',   'individual', 40.00, 'Fully automated',         'In progress',  'active', '2026-03-31'),
    (1, 10, 2, 'Launch Q1 Marketing Campaign',    'Execute multi-channel campaign for product launch',     'team',       50.00, '50,000 impressions',      '35,000',       'active', '2026-03-31'),
    (1, 11, 2, 'Grow Social Media Engagement',    'Increase social media engagement rate by 15%',          'individual', 30.00, '15% increase',            '8% increase',  'active', '2026-03-31')
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 21. NOTIFICATIONS (recent)
-- =====================================================================
INSERT INTO notifications (company_id, user_id, title, message, category, entity_type, entity_id, is_read, read_at) VALUES
    (1, 1,  'New Leave Request',           'Jasmine Villanueva has filed a sick leave request for March 10-11, 2026.',                'leave',       'leave_request', 2, false, NULL),
    (1, 1,  'Overtime Request Pending',     'Patricia Bautista has submitted an overtime request for March 4, 2026.',                  'approval',    'overtime_request', 2, false, NULL),
    (1, 3,  'Leave Request Approved',       'Your service incentive leave for March 5 has been approved by Maria Reyes.',             'leave',       'leave_request', 3, true,  '2026-03-03 15:00:00+08'),
    (1, 5,  'Payroll Processed',            'February 2026 2nd half payroll has been processed. Check your payslip.',                  'payroll',     'payroll_cycle',  4, true,  '2026-02-28 12:00:00+08'),
    (1, 2,  'Performance Review Due',       'Please complete your Q1 2026 self-review by March 31, 2026.',                           'performance', 'review_cycle',   2, true,  '2026-03-01 09:00:00+08'),
    (1, 1,  'Loan Request',                 'Kevin Delos Reyes has submitted a cash advance request for PHP 10,000.',                'loan',        'loan',           4, false, NULL),
    (1, 1,  'Employee Leave Request',       'Grace Soriano has filed an emergency leave request for March 7, 2026.',                 'leave',       'leave_request',  5, false, NULL),
    (1, 13, 'New Announcement',             'New announcement: Q1 2026 Town Hall Meeting. Please check the announcements page.',     'info',        NULL,             NULL, false, NULL)
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 22. TRAINING & CERTIFICATIONS
-- =====================================================================
INSERT INTO trainings (id, company_id, title, description, trainer, training_type, start_date, end_date, max_participants, status, created_by) VALUES
    (1, 1, 'Philippine Labor Law Essentials',       'Overview of key labor laws affecting HR and payroll',     'Atty. Marcos Rivera',  'external', '2026-02-15', '2026-02-15', 30, 'completed', 1),
    (2, 1, 'Data Privacy Act Compliance',           'RA 10173 compliance training for all employees',         'DPO Maria Santos',     'internal', '2026-03-10', '2026-03-10', 50, 'scheduled', 1),
    (3, 1, 'Advanced Go Programming Workshop',      '3-day intensive Go programming for engineering team',    'Tech Lead External',   'external', '2026-03-20', '2026-03-22', 10, 'scheduled', 1),
    (4, 1, 'Fire Safety and Emergency Procedures',  'Annual fire drill and emergency response training',      'BFP Coordinator',      'internal', '2026-04-05', '2026-04-05', 50, 'scheduled', 1)
ON CONFLICT DO NOTHING;

SELECT setval('trainings_id_seq', GREATEST((SELECT MAX(id) FROM trainings), 4));

-- Training participants
INSERT INTO training_participants (training_id, employee_id, status, score, feedback, completed_at) VALUES
    (1, 6,  'completed', 92.5, 'Very informative, especially the new DOLE orders section.', '2026-02-15 17:00:00+08'),
    (1, 7,  'completed', 88.0, 'Good overview. Would like a deeper dive into dispute resolution.', '2026-02-15 17:00:00+08'),
    (1, 8,  'completed', 95.0, 'Excellent presentation on tax compliance aspects.', '2026-02-15 17:00:00+08'),
    (1, 1,  'completed', 85.0, NULL, '2026-02-15 17:00:00+08'),
    (2, 1,  'enrolled',  NULL, NULL, NULL),
    (2, 6,  'enrolled',  NULL, NULL, NULL),
    (2, 7,  'enrolled',  NULL, NULL, NULL),
    (2, 14, 'enrolled',  NULL, NULL, NULL),
    (3, 1,  'enrolled',  NULL, NULL, NULL),
    (3, 2,  'enrolled',  NULL, NULL, NULL),
    (3, 3,  'enrolled',  NULL, NULL, NULL),
    (3, 4,  'enrolled',  NULL, NULL, NULL),
    (3, 5,  'enrolled',  NULL, NULL, NULL),
    (3, 16, 'enrolled',  NULL, NULL, NULL)
ON CONFLICT DO NOTHING;

-- Certifications
INSERT INTO certifications (company_id, employee_id, name, issuing_body, credential_id, issue_date, expiry_date, status) VALUES
    (1, 1,  'AWS Solutions Architect Associate',    'Amazon Web Services',     'AWS-SAA-2024-001', '2024-06-15', '2027-06-15', 'active'),
    (1, 2,  'Certified Kubernetes Administrator',   'Cloud Native Computing',  'CKA-2025-002',     '2025-03-20', '2028-03-20', 'active'),
    (1, 5,  'ISTQB Advanced Test Analyst',          'ISTQB',                   'ISTQB-ATA-003',    '2023-11-10', '2026-11-10', 'active'),
    (1, 6,  'CHRP - Certified HR Professional',     'PMAP',                    'CHRP-2024-004',    '2024-01-15', '2027-01-15', 'active'),
    (1, 8,  'CPA - Certified Public Accountant',    'PRC',                     'CPA-2020-005',     '2020-09-01', NULL,          'active')
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 23. BENEFIT PLANS & ENROLLMENTS
-- =====================================================================
INSERT INTO benefit_plans (id, company_id, name, category, description, provider, employer_share, employee_share, coverage_amount, eligibility_type, eligibility_months, is_active) VALUES
    (1, 1, 'PhilCare HMO Gold',       'medical',        'Comprehensive HMO with 1 dependent', 'PhilCare',              2500, 500,  200000, 'after_probation', 3,  true),
    (2, 1, 'Sun Life Group Insurance', 'life_insurance', 'Group term life insurance',           'Sun Life Philippines',  800,  0,    500000, 'regular',         6,  true),
    (3, 1, 'Dental Coverage',          'dental',         'Annual dental checkup and cleaning',  'MetroDental',           500,  200,  15000,  'all',             0,  true),
    (4, 1, 'Rice Allowance',           'allowance',      'Monthly rice subsidy',                NULL,                    1500, 0,    NULL,   'all',             0,  true)
ON CONFLICT DO NOTHING;

SELECT setval('benefit_plans_id_seq', GREATEST((SELECT MAX(id) FROM benefit_plans), 4));

-- Enrollments for regular employees in HMO
INSERT INTO benefit_enrollments (company_id, employee_id, plan_id, status, enrollment_date, effective_date, employer_share, employee_share)
SELECT 1, e.id, 1, 'active', e.hire_date + INTERVAL '3 months', e.hire_date + INTERVAL '3 months', 2500, 500
FROM employees e
WHERE e.company_id = 1 AND e.employment_type = 'regular' AND e.status = 'active' AND e.id <= 17
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 24. COMPANY POLICIES
-- =====================================================================
INSERT INTO company_policies (id, company_id, title, content, category, version, effective_date, requires_acknowledgment, is_active, created_by) VALUES
    (1, 1, 'Code of Conduct',
     'All employees are expected to maintain the highest standards of professional conduct. This includes:\n\n1. Treating all colleagues with respect and dignity\n2. Maintaining confidentiality of company information\n3. Avoiding conflicts of interest\n4. Following all applicable laws and regulations\n5. Reporting any violations through proper channels\n\nViolations may result in disciplinary action up to and including termination.',
     'code_of_conduct', 1, '2025-01-01', true, true, 1),
    (2, 1, 'Data Privacy Policy',
     'In compliance with Republic Act 10173 (Data Privacy Act of 2012), Demo Company is committed to protecting the personal data of its employees, clients, and partners.\n\nKey Points:\n1. Personal data is collected only for legitimate business purposes\n2. Data is stored securely with access controls\n3. Employees must report data breaches within 24 hours\n4. Data subjects have the right to access, correct, and delete their data\n5. Regular privacy impact assessments are conducted\n\nData Protection Officer: Maria Reyes (maria.reyes@demo.com)',
     'data_privacy', 1, '2025-01-01', true, true, 1),
    (3, 1, 'Anti-Sexual Harassment Policy',
     'Demo Company has zero tolerance for sexual harassment in accordance with RA 7877 and the Safe Spaces Act (RA 11313).\n\nCovered acts include:\n1. Unwelcome sexual advances\n2. Requests for sexual favors\n3. Verbal or physical conduct of a sexual nature\n4. Gender-based online harassment\n\nReporting: Employees can report incidents to HR, their manager, or anonymously through the grievance system.\n\nInvestigation and resolution within 30 days of complaint.',
     'anti_harassment', 1, '2025-01-01', true, true, 1)
ON CONFLICT DO NOTHING;

SELECT setval('company_policies_id_seq', GREATEST((SELECT MAX(id) FROM company_policies), 3));

-- =====================================================================
-- 25. ONBOARDING TEMPLATES
-- =====================================================================
INSERT INTO onboarding_templates (id, company_id, workflow_type, title, description, sort_order, is_required, assignee_role, due_days, is_active) VALUES
    (1,  1, 'onboarding', 'Submit Government IDs',        'Submit copies of TIN, SSS, PhilHealth, Pag-IBIG IDs',   1, true,  'employee', 3,  true),
    (2,  1, 'onboarding', 'Complete Employee Profile',     'Fill out personal information in the HRIS portal',       2, true,  'employee', 3,  true),
    (3,  1, 'onboarding', 'Sign Employment Contract',      'Review and sign the employment contract',                3, true,  'hr',       1,  true),
    (4,  1, 'onboarding', 'Setup IT Accounts',             'Create email, Slack, and system access accounts',        4, true,  'it',       2,  true),
    (5,  1, 'onboarding', 'Orient on Company Policies',    'Review code of conduct, data privacy, and HR policies',  5, true,  'hr',       5,  true),
    (6,  1, 'onboarding', 'Assign Workstation',            'Prepare desk, laptop, and office supplies',              6, true,  'it',       2,  true),
    (7,  1, 'onboarding', 'Meet the Team',                 'Introduction to department members and key contacts',    7, false, 'manager',  3,  true),
    (8,  1, 'offboarding', 'Return Company Equipment',     'Return laptop, ID, access cards, and keys',             1, true,  'employee', 5,  true),
    (9,  1, 'offboarding', 'Exit Interview',               'Conduct exit interview with HR',                        2, true,  'hr',       10, true),
    (10, 1, 'offboarding', 'Disable System Access',        'Revoke all system and email access',                    3, true,  'it',       1,  true)
ON CONFLICT DO NOTHING;

SELECT setval('onboarding_templates_id_seq', GREATEST((SELECT MAX(id) FROM onboarding_templates), 10));

-- Onboarding tasks for probationary employee (Mark Aquino, emp_id=4)
INSERT INTO onboarding_tasks (company_id, employee_id, template_id, workflow_type, title, is_required, assignee_role, due_date, status, completed_by, completed_at, sort_order) VALUES
    (1, 4, 1, 'onboarding', 'Submit Government IDs',        true, 'employee', '2024-09-04', 'completed', 9, '2024-09-03 10:00:00+08', 1),
    (1, 4, 2, 'onboarding', 'Complete Employee Profile',     true, 'employee', '2024-09-04', 'completed', 9, '2024-09-02 14:00:00+08', 2),
    (1, 4, 3, 'onboarding', 'Sign Employment Contract',      true, 'hr',       '2024-09-02', 'completed', 3, '2024-09-01 09:00:00+08', 3),
    (1, 4, 4, 'onboarding', 'Setup IT Accounts',             true, 'it',       '2024-09-03', 'completed', 1, '2024-09-01 11:00:00+08', 4),
    (1, 4, 5, 'onboarding', 'Orient on Company Policies',    true, 'hr',       '2024-09-06', 'completed', 3, '2024-09-05 16:00:00+08', 5),
    (1, 4, 6, 'onboarding', 'Assign Workstation',            true, 'it',       '2024-09-03', 'completed', 1, '2024-09-01 10:00:00+08', 6),
    (1, 4, 7, 'onboarding', 'Meet the Team',                 false,'manager',  '2024-09-04', 'completed', 2, '2024-09-02 10:00:00+08', 7)
ON CONFLICT DO NOTHING;

-- Onboarding tasks for new probationary employee (Angelica Navarro, emp_id=15) - in progress
INSERT INTO onboarding_tasks (company_id, employee_id, template_id, workflow_type, title, is_required, assignee_role, due_date, status, sort_order) VALUES
    (1, 15, 1, 'onboarding', 'Submit Government IDs',        true, 'employee', '2025-01-18', 'completed', 1),
    (1, 15, 2, 'onboarding', 'Complete Employee Profile',     true, 'employee', '2025-01-18', 'completed', 2),
    (1, 15, 3, 'onboarding', 'Sign Employment Contract',      true, 'hr',       '2025-01-16', 'completed', 3),
    (1, 15, 4, 'onboarding', 'Setup IT Accounts',             true, 'it',       '2025-01-17', 'in_progress', 4),
    (1, 15, 5, 'onboarding', 'Orient on Company Policies',    true, 'hr',       '2025-01-20', 'pending',  5),
    (1, 15, 6, 'onboarding', 'Assign Workstation',            true, 'it',       '2025-01-17', 'completed', 6),
    (1, 15, 7, 'onboarding', 'Meet the Team',                 false,'manager',  '2025-01-18', 'pending',  7)
ON CONFLICT DO NOTHING;

-- Offboarding tasks for resigned employee (Diana Ramos, emp_id=17)
INSERT INTO onboarding_tasks (company_id, employee_id, template_id, workflow_type, title, is_required, assignee_role, due_date, status, sort_order) VALUES
    (1, 17, 8,  'offboarding', 'Return Company Equipment',     true, 'employee', '2026-01-05', 'completed', 1),
    (1, 17, 9,  'offboarding', 'Exit Interview',               true, 'hr',       '2026-01-10', 'completed', 2),
    (1, 17, 10, 'offboarding', 'Disable System Access',        true, 'it',       '2026-01-01', 'completed', 3)
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 26. GEOFENCE LOCATIONS
-- =====================================================================
INSERT INTO geofence_locations (id, company_id, name, address, latitude, longitude, radius_meters, is_active, enforce_on_clock_in, enforce_on_clock_out, created_by) VALUES
    (1, 1, 'Main Office - Makati',      '123 Rizal Avenue, Brgy. San Miguel, Makati City', 14.5547000, 121.0244000, 200, true,  true, false, 1),
    (2, 1, 'Satellite Office - BGC',     'Unit 5A, High Street South, BGC, Taguig City',   14.5503000, 121.0502000, 150, true,  true, false, 1),
    (3, 1, 'Client Site - Ortigas',      'Ortigas Center, Pasig City',                      14.5872000, 121.0615000, 300, false, true, false, 1)
ON CONFLICT DO NOTHING;

SELECT setval('geofence_locations_id_seq', GREATEST((SELECT MAX(id) FROM geofence_locations), 3));

-- =====================================================================
-- 27. EXPENSE CATEGORIES & CLAIMS
-- =====================================================================
INSERT INTO expense_categories (id, company_id, name, description, max_amount, requires_receipt, is_active) VALUES
    (1, 1, 'Transportation',    'Taxi, Grab, parking fees',                5000.00,  true,  true),
    (2, 1, 'Meals & Entertainment', 'Client meals and team lunches',       3000.00,  true,  true),
    (3, 1, 'Office Supplies',   'Stationery, printer ink, etc.',           2000.00,  true,  true),
    (4, 1, 'Communication',     'Mobile load, internet for WFH',          1500.00,  false, true),
    (5, 1, 'Training & Seminars','Course fees, conference registration',   50000.00, true,  true)
ON CONFLICT DO NOTHING;

SELECT setval('expense_categories_id_seq', GREATEST((SELECT MAX(id) FROM expense_categories), 5));

INSERT INTO expense_claims (id, company_id, employee_id, claim_number, category_id, description, amount, currency, expense_date, status, submitted_at, approver_id, approved_at, notes) VALUES
    (1, 1, 2,  'EXP-2026-0001', 1, 'Grab ride to client meeting - Ortigas',     450.00, 'PHP', '2026-02-25', 'approved',  '2026-02-26 09:00:00+08', 1, '2026-02-26 14:00:00+08', NULL),
    (2, 1, 10, 'EXP-2026-0002', 2, 'Client lunch meeting - Greenbelt',          2800.00,'PHP', '2026-03-01', 'submitted', '2026-03-02 10:00:00+08', NULL, NULL, '3 pax including client from ABC Corp'),
    (3, 1, 14, 'EXP-2026-0003', 3, 'Printer ink and bond paper for office',     1250.00,'PHP', '2026-02-28', 'approved',  '2026-03-01 08:00:00+08', 1, '2026-03-01 10:00:00+08', NULL)
ON CONFLICT DO NOTHING;

SELECT setval('expense_claims_id_seq', GREATEST((SELECT MAX(id) FROM expense_claims), 3));

-- =====================================================================
-- 28. GRIEVANCE CASES
-- =====================================================================
INSERT INTO grievance_cases (id, company_id, employee_id, case_number, category, subject, description, severity, status, assigned_to, is_anonymous) VALUES
    (1, 1, 7, 'GRV-2026-001', 'working_conditions', 'Office Temperature Issues',
     'The air conditioning in the 3rd floor engineering area has been set too cold consistently, causing discomfort and affecting productivity. Multiple verbal complaints have been made to facilities but no action taken.',
     'low', 'under_review', 1, false),
    (2, 1, 3, 'GRV-2026-002', 'workplace_safety', 'Emergency Exit Blocked',
     'The emergency exit near the pantry on the 2nd floor has been partially blocked by storage boxes for the past week. This is a fire safety hazard that needs immediate attention.',
     'high', 'open', NULL, true)
ON CONFLICT DO NOTHING;

SELECT setval('grievance_cases_id_seq', GREATEST((SELECT MAX(id) FROM grievance_cases), 2));

INSERT INTO grievance_comments (grievance_id, user_id, comment, is_internal) VALUES
    (1, 1, 'Will coordinate with facilities management to address the temperature issue. Targeting resolution by end of week.', true),
    (1, 3, 'Thank you for looking into this. The team would really appreciate it.', false)
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 29. DOCUMENT CATEGORIES & REQUIREMENTS
-- =====================================================================
INSERT INTO document_categories (id, company_id, name, slug, description, sort_order, is_system) VALUES
    (1, 1, 'Government IDs',       'government-ids',      'Government-issued identification documents',    1, true),
    (2, 1, 'Employment Documents',  'employment-docs',     'Contracts, offer letters, and employment records', 2, true),
    (3, 1, 'Certificates',          'certificates',        'Educational and professional certificates',     3, true),
    (4, 1, 'Medical Records',       'medical-records',     'Pre-employment and annual medical exams',       4, true),
    (5, 1, 'Tax Documents',         'tax-documents',       'BIR forms and tax-related documents',           5, true)
ON CONFLICT DO NOTHING;

SELECT setval('document_categories_id_seq', GREATEST((SELECT MAX(id) FROM document_categories), 5));

INSERT INTO document_requirements (company_id, category_id, document_name, is_required, applies_to, expiry_months) VALUES
    (1, 1, 'TIN ID',                   true,  'all',          NULL),
    (1, 1, 'SSS ID',                   true,  'all',          NULL),
    (1, 1, 'PhilHealth ID',            true,  'all',          NULL),
    (1, 1, 'Pag-IBIG ID',             true,  'all',          NULL),
    (1, 2, 'Employment Contract',      true,  'all',          NULL),
    (1, 2, 'NBI Clearance',            true,  'all',          12),
    (1, 3, 'Diploma/TOR',             true,  'all',          NULL),
    (1, 4, 'Pre-Employment Medical',   true,  'all',          12),
    (1, 5, 'BIR Form 2316',           true,  'all',          NULL),
    (1, 5, 'BIR Form 1902',           true,  'all',          NULL)
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 30. TAX FILINGS (2026)
-- =====================================================================
INSERT INTO tax_filings (company_id, filing_type, period_type, period_year, period_month, period_quarter, due_date, status, amount) VALUES
    (1, 'bir_1601c',  'monthly',    2026, 1, NULL, '2026-02-10', 'filed',   45000.00),
    (1, 'bir_1601c',  'monthly',    2026, 2, NULL, '2026-03-10', 'filed',   47500.00),
    (1, 'bir_1601c',  'monthly',    2026, 3, NULL, '2026-04-10', 'pending', 0),
    (1, 'sss_r3',     'monthly',    2026, 1, NULL, '2026-02-15', 'filed',   65000.00),
    (1, 'sss_r3',     'monthly',    2026, 2, NULL, '2026-03-15', 'filed',   65000.00),
    (1, 'philhealth_rf1', 'monthly',2026, 1, NULL, '2026-02-15', 'filed',   35000.00),
    (1, 'philhealth_rf1', 'monthly',2026, 2, NULL, '2026-03-15', 'filed',   35000.00),
    (1, 'pagibig_ml1','monthly',    2026, 1, NULL, '2026-02-15', 'filed',   8000.00),
    (1, 'pagibig_ml1','monthly',    2026, 2, NULL, '2026-03-15', 'filed',   8000.00),
    (1, 'bir_0619e',  'monthly',    2026, 1, NULL, '2026-02-10', 'filed',   45000.00),
    (1, 'bir_0619e',  'monthly',    2026, 2, NULL, '2026-03-10', 'filed',   47500.00)
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 31. EMPLOYMENT HISTORY
-- =====================================================================
INSERT INTO employment_history (company_id, employee_id, action_type, effective_date, from_department_id, to_department_id, from_position_id, to_position_id, remarks, created_by) VALUES
    (1, 1,  'hire',       '2022-01-15', NULL, 1, NULL, 1,  'Hired as CTO',                                1),
    (1, 2,  'hire',       '2022-03-01', NULL, 1, NULL, 2,  'Hired as Senior Software Developer',           1),
    (1, 2,  'promotion',  '2025-01-01', 1,   1, 3,   2,   'Promoted from Junior to Senior Developer',      1),
    (1, 3,  'hire',       '2023-06-15', NULL, 1, NULL, 3,  'Hired as Junior Software Developer',           1),
    (1, 6,  'hire',       '2022-02-01', NULL, 2, NULL, 5,  'Hired as HR Manager',                          1),
    (1, 8,  'hire',       '2022-01-20', NULL, 3, NULL, 7,  'Hired as Finance Manager',                     1),
    (1, 10, 'hire',       '2022-04-01', NULL, 4, NULL, 9,  'Hired as Marketing Manager',                   1),
    (1, 12, 'hire',       '2022-05-01', NULL, 5, NULL, 11, 'Hired as Operations Manager',                  1),
    (1, 17, 'hire',       '2024-03-01', NULL, 4, NULL, 10, 'Hired as Marketing Associate',                 1),
    (1, 17, 'separation', '2025-12-31', 4,   NULL, 10, NULL, 'Resigned - pursuing further studies abroad', 1)
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 32. CONTRACT MILESTONES
-- =====================================================================
INSERT INTO contract_milestones (company_id, employee_id, milestone_type, milestone_date, days_remaining, status, notes) VALUES
    (1, 4,  'probation_ending',     '2025-03-01', 0,  'actioned',    'Regularized on time'),
    (1, 13, 'probation_ending',     '2024-12-01', 0,  'actioned',    'Regularization pending evaluation'),
    (1, 15, 'probation_ending',     '2025-07-15', 131,'pending',     'Probation ends July 2025 - schedule evaluation'),
    (1, 16, 'contract_expiring',    '2025-07-01', 117,'pending',     'Contractual engagement ends - decide renewal'),
    (1, 1,  'anniversary',          '2026-01-15', 0,  'acknowledged','4th work anniversary'),
    (1, 6,  'anniversary',          '2026-02-01', 0,  'acknowledged','4th work anniversary')
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 33. ATTENDANCE CORRECTIONS
-- =====================================================================
INSERT INTO attendance_corrections (company_id, employee_id, correction_date, original_clock_in, requested_clock_in, requested_clock_out, reason, status, reviewed_by, reviewed_at, review_note) VALUES
    (1, 11, '2026-03-02',
     '2026-03-02 08:35:00+08',
     '2026-03-02 08:00:00+08',
     '2026-03-02 17:00:00+08',
     'System glitch - I was in the office at 8:00 AM but the time-in only registered at 8:35.',
     'pending', NULL, NULL, NULL),
    (1, 3,  '2026-02-28',
     '2026-02-28 08:05:00+08',
     '2026-02-28 08:05:00+08',
     '2026-02-28 18:30:00+08',
     'Forgot to clock out. Worked until 6:30 PM on the sprint demo preparation.',
     'approved', 1, '2026-03-01 09:00:00+08', 'Verified with building access logs. Approved.')
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 34. COST CENTERS
-- =====================================================================
INSERT INTO cost_centers (id, company_id, code, name, is_active) VALUES
    (1, 1, 'CC-ENG',  'Engineering Cost Center', true),
    (2, 1, 'CC-CORP', 'Corporate Cost Center',   true),
    (3, 1, 'CC-OPS',  'Operations Cost Center',  true)
ON CONFLICT DO NOTHING;

SELECT setval('cost_centers_id_seq', GREATEST((SELECT MAX(id) FROM cost_centers), 3));

-- =====================================================================
-- 35. ACTIVITY LOG (sample system events)
-- =====================================================================
INSERT INTO audit_logs (company_id, user_id, action, entity_type, entity_id, new_values) VALUES
    (1, 1, 'login',   'user',           1,  '{"ip":"192.168.1.100"}'::jsonb),
    (1, 1, 'create',  'payroll_cycle',  5,  '{"name":"March 2026 - 1st Half","status":"draft"}'::jsonb),
    (1, 1, 'approve', 'leave_request',  1,  '{"employee":"Carlo Mendoza","days":3}'::jsonb),
    (1, 3, 'create',  'leave_request',  3,  '{"type":"SIL","days":1}'::jsonb),
    (1, 1, 'approve', 'overtime_request',1, '{"employee":"Carlo Mendoza","hours":3}'::jsonb)
ON CONFLICT DO NOTHING;

-- =====================================================================
-- DONE - Summary of seeded data:
-- - 1 company with full details
-- - 18 users (1 admin, 5 managers, 12 employees)
-- - 7 departments with heads assigned
-- - 14 positions across departments
-- - 18 employees (13 regular, 3 probationary, 1 contractual, 1 separated)
-- - 17 employee profiles with government IDs & bank info
-- - 4 shifts + 2 schedule templates
-- - 10 leave types (PH statutory + company)
-- - Leave balances for 2026
-- - 6 leave requests (approved, pending, rejected, cancelled)
-- - ~7 days of attendance logs for active employees
-- - 3 overtime requests
-- - 8 salary components + 17 salary records
-- - 5 payroll cycles + 4 completed runs + payroll items
-- - 5 loan types + 4 loans with payment history
-- - 3 announcements
-- - 5 company-specific knowledge articles
-- - 2 review cycles + reviews + 6 goals
-- - 8 notifications
-- - 4 trainings + participants + 5 certifications
-- - 4 benefit plans + enrollments
-- - 3 company policies
-- - 10 onboarding templates + tasks for 3 employees
-- - 3 geofence locations
-- - 5 expense categories + 3 claims
-- - 2 grievance cases
-- - 5 document categories + 10 requirements
-- - 11 tax filing records for 2026
-- - Employment history records
-- - 6 contract milestones
-- - 2 attendance corrections
-- - 3 cost centers
-- - 5 audit log entries
-- =====================================================================


-- +goose Down
-- Clean up all seed data in reverse dependency order
DELETE FROM grievance_comments WHERE grievance_id IN (SELECT id FROM grievance_cases WHERE company_id = 1);
DELETE FROM grievance_cases WHERE company_id = 1;
DELETE FROM expense_claims WHERE company_id = 1;
DELETE FROM expense_categories WHERE company_id = 1;
DELETE FROM policy_acknowledgments WHERE company_id = 1;
DELETE FROM company_policies WHERE company_id = 1;
DELETE FROM attendance_corrections WHERE company_id = 1;
DELETE FROM benefit_claims WHERE company_id = 1;
DELETE FROM benefit_dependents WHERE company_id = 1;
DELETE FROM benefit_enrollments WHERE company_id = 1;
DELETE FROM benefit_plans WHERE company_id = 1;
DELETE FROM training_participants WHERE training_id IN (SELECT id FROM trainings WHERE company_id = 1);
DELETE FROM trainings WHERE company_id = 1;
DELETE FROM certifications WHERE company_id = 1;
DELETE FROM goals WHERE company_id = 1;
DELETE FROM performance_reviews WHERE company_id = 1;
DELETE FROM review_cycles WHERE company_id = 1;
DELETE FROM notifications WHERE company_id = 1;
DELETE FROM announcements WHERE company_id = 1;
DELETE FROM loan_payments WHERE loan_id IN (SELECT id FROM loans WHERE company_id = 1);
DELETE FROM loans WHERE company_id = 1;
DELETE FROM loan_types WHERE company_id = 1;
DELETE FROM payroll_items WHERE run_id IN (SELECT id FROM payroll_runs WHERE company_id = 1);
DELETE FROM payslips WHERE company_id = 1;
DELETE FROM payroll_runs WHERE company_id = 1;
DELETE FROM payroll_cycles WHERE company_id = 1;
DELETE FROM employee_salary_components WHERE employee_salary_id IN (SELECT id FROM employee_salaries WHERE company_id = 1);
DELETE FROM employee_salaries WHERE company_id = 1;
DELETE FROM salary_components WHERE company_id = 1;
DELETE FROM salary_structures WHERE company_id = 1;
DELETE FROM overtime_requests WHERE company_id = 1;
DELETE FROM leave_requests WHERE company_id = 1;
DELETE FROM leave_balances WHERE company_id = 1;
DELETE FROM leave_encashments WHERE company_id = 1;
DELETE FROM leave_types WHERE company_id = 1;
DELETE FROM attendance_logs WHERE company_id = 1;
DELETE FROM employee_schedule_assignments WHERE company_id = 1;
DELETE FROM schedule_template_days WHERE template_id IN (SELECT id FROM schedule_templates WHERE company_id = 1);
DELETE FROM schedule_templates WHERE company_id = 1;
DELETE FROM work_schedules WHERE company_id = 1;
DELETE FROM shifts WHERE company_id = 1;
DELETE FROM onboarding_tasks WHERE company_id = 1;
DELETE FROM onboarding_templates WHERE company_id = 1;
DELETE FROM contract_milestones WHERE company_id = 1;
DELETE FROM tax_filings WHERE company_id = 1;
DELETE FROM remittance_records WHERE company_id = 1;
DELETE FROM document_requirements WHERE company_id = 1;
DELETE FROM employee_documents WHERE company_id = 1;
DELETE FROM document_categories WHERE company_id = 1;
DELETE FROM audit_logs WHERE company_id = 1;
DELETE FROM approval_workflows WHERE company_id = 1;
DELETE FROM geofence_locations WHERE company_id = 1;
DELETE FROM employment_history WHERE company_id = 1;
DELETE FROM employee_profiles WHERE employee_id IN (SELECT id FROM employees WHERE company_id = 1);
UPDATE departments SET head_employee_id = NULL WHERE company_id = 1;
DELETE FROM employees WHERE company_id = 1;
DELETE FROM knowledge_articles WHERE company_id = 1;
DELETE FROM cost_centers WHERE company_id = 1;
DELETE FROM positions WHERE company_id = 1;
DELETE FROM departments WHERE company_id = 1;
DELETE FROM users WHERE company_id = 1 AND id > 1;
