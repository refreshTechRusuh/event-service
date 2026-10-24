-- migrations/001_initial_schema.sql
-- Event Service: event dan ticket_category

CREATE TABLE IF NOT EXISTS event (
    id            UUID PRIMARY KEY,
    name          TEXT NOT NULL,
    sale_opens_at TIMESTAMPTZ,
    status        TEXT NOT NULL DEFAULT 'draft',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT event_status_check CHECK (status IN ('draft','on_sale','sold_out','closed'))
);

CREATE TABLE IF NOT EXISTS ticket_category (
    id         UUID PRIMARY KEY,
    event_id   UUID NOT NULL REFERENCES event(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    price      BIGINT NOT NULL,
    capacity   INT NOT NULL,
    available  INT NOT NULL CHECK (available >= 0),
    CONSTRAINT tc_available_lte_capacity CHECK (available <= capacity)
);

CREATE INDEX IF NOT EXISTS idx_event_status ON event(status);
CREATE INDEX IF NOT EXISTS idx_tc_event_id ON ticket_category(event_id);
