CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT UNIQUE NOT NULL,
    password_hash TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE feature_requests (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id   UUID NOT NULL REFERENCES users(id),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    vote_count  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE votes (
    user_id    UUID NOT NULL REFERENCES users(id),
    request_id UUID NOT NULL REFERENCES feature_requests(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, request_id)
);

-- "Top" ranking + keyset pagination
CREATE INDEX idx_fr_top     ON feature_requests (vote_count DESC, created_at DESC, id DESC);
-- "New" + recency tie-breaks
CREATE INDEX idx_fr_created ON feature_requests (created_at DESC, id DESC);
-- audit / per-request vote scans
CREATE INDEX idx_votes_request ON votes (request_id);
