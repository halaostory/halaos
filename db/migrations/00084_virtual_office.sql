-- +goose Up
CREATE TABLE virtual_office_config (
    company_id   BIGINT PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    template     TEXT NOT NULL DEFAULT 'small',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE virtual_office_seats (
    id                BIGSERIAL PRIMARY KEY,
    company_id        BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    employee_id       BIGINT NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    floor             INT NOT NULL DEFAULT 1,
    zone              TEXT NOT NULL DEFAULT 'desk-a',
    seat_x            INT NOT NULL,
    seat_y            INT NOT NULL,
    avatar_type       TEXT NOT NULL DEFAULT 'person_1',
    avatar_color      TEXT NOT NULL DEFAULT '#4A90D9',
    custom_status     TEXT,
    custom_emoji      TEXT,
    manual_status     TEXT,
    meeting_room_zone TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_vo_seats_company_employee ON virtual_office_seats(company_id, employee_id);
CREATE UNIQUE INDEX idx_vo_seats_company_position ON virtual_office_seats(company_id, floor, seat_x, seat_y);
CREATE INDEX idx_vo_seats_company ON virtual_office_seats(company_id);

-- +goose Down
DROP TABLE IF EXISTS virtual_office_seats;
DROP TABLE IF EXISTS virtual_office_config;
