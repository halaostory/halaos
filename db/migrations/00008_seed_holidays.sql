-- +goose Up
-- Seed Philippine holidays for 2025 and 2026
-- These are nationwide holidays; companies can add more

-- Note: company_id = 1 (demo company). For multi-tenant, seed per company on registration.

-- 2025 Regular Holidays
INSERT INTO holidays (company_id, name, holiday_date, holiday_type, year, is_nationwide) VALUES
(1, 'New Year''s Day', '2025-01-01', 'regular', 2025, true),
(1, 'Araw ng Kagitingan', '2025-04-09', 'regular', 2025, true),
(1, 'Maundy Thursday', '2025-04-17', 'regular', 2025, true),
(1, 'Good Friday', '2025-04-18', 'regular', 2025, true),
(1, 'Labor Day', '2025-05-01', 'regular', 2025, true),
(1, 'Independence Day', '2025-06-12', 'regular', 2025, true),
(1, 'National Heroes Day', '2025-08-25', 'regular', 2025, true),
(1, 'Bonifacio Day', '2025-11-30', 'regular', 2025, true),
(1, 'Christmas Day', '2025-12-25', 'regular', 2025, true),
(1, 'Rizal Day', '2025-12-30', 'regular', 2025, true),
(1, 'Eid''ul Fitr', '2025-03-31', 'regular', 2025, true),
(1, 'Eid''ul Adha', '2025-06-07', 'regular', 2025, true)
ON CONFLICT DO NOTHING;

-- 2025 Special Non-Working Holidays
INSERT INTO holidays (company_id, name, holiday_date, holiday_type, year, is_nationwide) VALUES
(1, 'EDSA People Power Revolution Anniversary', '2025-02-25', 'special_non_working', 2025, true),
(1, 'Black Saturday', '2025-04-19', 'special_non_working', 2025, true),
(1, 'Ninoy Aquino Day', '2025-08-21', 'special_non_working', 2025, true),
(1, 'All Saints'' Day', '2025-11-01', 'special_non_working', 2025, true),
(1, 'Feast of Immaculate Conception', '2025-12-08', 'special_non_working', 2025, true),
(1, 'Last Day of the Year', '2025-12-31', 'special_non_working', 2025, true),
(1, 'Chinese New Year', '2025-01-29', 'special_non_working', 2025, true)
ON CONFLICT DO NOTHING;

-- 2026 Regular Holidays
INSERT INTO holidays (company_id, name, holiday_date, holiday_type, year, is_nationwide) VALUES
(1, 'New Year''s Day', '2026-01-01', 'regular', 2026, true),
(1, 'Araw ng Kagitingan', '2026-04-09', 'regular', 2026, true),
(1, 'Maundy Thursday', '2026-04-02', 'regular', 2026, true),
(1, 'Good Friday', '2026-04-03', 'regular', 2026, true),
(1, 'Labor Day', '2026-05-01', 'regular', 2026, true),
(1, 'Independence Day', '2026-06-12', 'regular', 2026, true),
(1, 'National Heroes Day', '2026-08-31', 'regular', 2026, true),
(1, 'Bonifacio Day', '2026-11-30', 'regular', 2026, true),
(1, 'Christmas Day', '2026-12-25', 'regular', 2026, true),
(1, 'Rizal Day', '2026-12-30', 'regular', 2026, true),
(1, 'Eid''ul Fitr', '2026-03-20', 'regular', 2026, true),
(1, 'Eid''ul Adha', '2026-05-27', 'regular', 2026, true)
ON CONFLICT DO NOTHING;

-- 2026 Special Non-Working Holidays
INSERT INTO holidays (company_id, name, holiday_date, holiday_type, year, is_nationwide) VALUES
(1, 'EDSA People Power Revolution Anniversary', '2026-02-25', 'special_non_working', 2026, true),
(1, 'Black Saturday', '2026-04-04', 'special_non_working', 2026, true),
(1, 'Ninoy Aquino Day', '2026-08-21', 'special_non_working', 2026, true),
(1, 'All Saints'' Day', '2026-11-01', 'special_non_working', 2026, true),
(1, 'Feast of Immaculate Conception', '2026-12-08', 'special_non_working', 2026, true),
(1, 'Last Day of the Year', '2026-12-31', 'special_non_working', 2026, true),
(1, 'Chinese New Year', '2026-02-17', 'special_non_working', 2026, true)
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM holidays WHERE company_id = 1 AND year IN (2025, 2026);
