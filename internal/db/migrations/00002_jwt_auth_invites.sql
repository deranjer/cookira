-- +goose Up

-- Browser logins are now short-lived JWT access tokens plus rotating opaque
-- refresh tokens, so the DB-backed sessions table goes away (existing
-- logins simply have to sign in again).
DROP TABLE sessions;

-- Refresh tokens are random and stored only as a sha256 hash. Each is
-- single-use: redeeming one revokes it and issues the next.
CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);

CREATE INDEX idx_refresh_tokens_user ON refresh_tokens (user_id);

-- Invite links: an admin creates one, shares the link, and the invitee picks
-- their own name and password. Stored hashed like every other token. An
-- invite may be bound to an email, and always carries the role to grant.
CREATE TABLE invites (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    token_hash text NOT NULL UNIQUE,
    email      text,
    role       text NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    used_by    uuid REFERENCES users(id) ON DELETE SET NULL
);

-- +goose Down
DROP TABLE invites;
DROP TABLE refresh_tokens;
CREATE TABLE sessions (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions (user_id);
