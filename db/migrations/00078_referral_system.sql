-- +goose Up

-- Each company gets a unique referral code for sharing.
ALTER TABLE companies ADD COLUMN referral_code VARCHAR(20) UNIQUE;
ALTER TABLE companies ADD COLUMN referred_by_code VARCHAR(20);
ALTER TABLE companies ADD COLUMN referral_reward_claimed BOOLEAN NOT NULL DEFAULT false;

-- Track referral events.
CREATE TABLE referral_events (
    id            BIGSERIAL    PRIMARY KEY,
    referrer_company_id BIGINT NOT NULL REFERENCES companies(id),
    referred_company_id BIGINT NOT NULL REFERENCES companies(id),
    referral_code VARCHAR(20)  NOT NULL,
    status        VARCHAR(20)  NOT NULL DEFAULT 'pending',
    reward_type   VARCHAR(50),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    activated_at  TIMESTAMPTZ,
    UNIQUE(referred_company_id)
);

CREATE INDEX idx_referral_events_referrer ON referral_events(referrer_company_id);

-- +goose Down
DROP TABLE IF EXISTS referral_events;
ALTER TABLE companies DROP COLUMN IF EXISTS referral_reward_claimed;
ALTER TABLE companies DROP COLUMN IF EXISTS referred_by_code;
ALTER TABLE companies DROP COLUMN IF EXISTS referral_code;
