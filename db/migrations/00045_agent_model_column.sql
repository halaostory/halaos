-- +goose Up
-- Add model column to agents table for per-agent LLM model routing.
-- Empty string means "use provider default" (claude-sonnet-4-5).
ALTER TABLE agents ADD COLUMN IF NOT EXISTS model VARCHAR(100) NOT NULL DEFAULT '';

-- Simple agents → Haiku (fast, cheap, good enough for lookups)
UPDATE agents SET model = 'claude-haiku-4-5-20251001'
WHERE slug IN ('attendance', 'leave', 'onboarding', 'general');

-- Complex agents → Sonnet (deeper reasoning for calculations & compliance)
UPDATE agents SET model = 'claude-sonnet-4-5-20250514'
WHERE slug IN ('payroll', 'compliance');

-- +goose Down
ALTER TABLE agents DROP COLUMN IF EXISTS model;
