-- Pick Me — full schema reference
-- This file is for documentation only. Migrations are run from migrations.go.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    google_id  TEXT        UNIQUE NOT NULL,
    name       TEXT        NOT NULL,
    email      TEXT        UNIQUE NOT NULL,
    avatar_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS friendships (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    receiver_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT        NOT NULL CHECK (status IN ('pending', 'accepted', 'declined')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT unique_friendship UNIQUE (requester_id, receiver_id)
);

CREATE INDEX IF NOT EXISTS idx_friendships_requester ON friendships(requester_id);
CREATE INDEX IF NOT EXISTS idx_friendships_receiver  ON friendships(receiver_id);

CREATE TABLE IF NOT EXISTS pickup_requests (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    receiver_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status        TEXT        NOT NULL CHECK (status IN ('pending', 'accepted', 'declined', 'cancelled', 'completed', 'timed_out')),
    requester_lat DOUBLE PRECISION NOT NULL,
    requester_lng DOUBLE PRECISION NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at  TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_pickup_requests_requester ON pickup_requests(requester_id);
CREATE INDEX IF NOT EXISTS idx_pickup_requests_receiver  ON pickup_requests(receiver_id);
CREATE INDEX IF NOT EXISTS idx_pickup_requests_status    ON pickup_requests(status);

CREATE TABLE IF NOT EXISTS notifications (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT        NOT NULL,
    payload    JSONB       NOT NULL,
    is_read    BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_is_read ON notifications(is_read);
