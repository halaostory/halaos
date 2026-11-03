-- Recalculating a cycle left the earlier runs "completed", so remittance
-- reports, BIR 2316, YTD and payroll history counted the cycle twice.
-- Keep only the latest run of each cycle and type; mark the rest.

-- +goose Up
UPDATE payroll_runs r SET status = 'superseded'
WHERE r.status <> 'superseded'
  AND EXISTS (SELECT 1 FROM payroll_runs n WHERE n.cycle_id = r.cycle_id AND n.run_type = r.run_type AND n.id > r.id);

-- +goose Down
SELECT 1;
