-- HalaPeople is HR-only: drop the virtual office, referral programme, NPS
-- survey, marketing drip e-mails and the external "brain" event bridge.

-- +goose Up
DROP TABLE IF EXISTS virtual_office_seats;
DROP TABLE IF EXISTS virtual_office_config;
DROP TABLE IF EXISTS referral_events;
DROP TABLE IF EXISTS drip_emails;
DROP TABLE IF EXISTS nps_feedback;
DROP TABLE IF EXISTS brain_outbox;
DROP TABLE IF EXISTS brain_links;
ALTER TABLE companies DROP COLUMN IF EXISTS referral_reward_claimed;
ALTER TABLE companies DROP COLUMN IF EXISTS referred_by_code;
ALTER TABLE companies DROP COLUMN IF EXISTS referral_code;

-- +goose Down
-- Irreversible: the removed features no longer exist in the codebase.
SELECT 1;
