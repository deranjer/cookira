-- +goose Up

-- Ids are uuidv7() (Postgres 18): time-ordered, so new rows append to the
-- index instead of scattering across it.

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    name          text NOT NULL,
    email         text NOT NULL UNIQUE,
    role          text NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    password_hash text,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- Logins are short-lived JWT access tokens plus rotating opaque refresh
-- tokens. Refresh tokens are random and stored only as a sha256 hash; each is
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

CREATE TABLE api_keys (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         text NOT NULL,
    key_hash     text NOT NULL UNIQUE,
    scopes       text[] NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz
);

-- Recipes. Ingredients and steps are JSONB documents rather than child
-- tables: they're always read and written as a whole, agents edit them as a
-- whole, and revisions snapshot them as a whole. Numeric fields use 0 for
-- "not specified" so patch semantics stay simple.
--
--   ingredients: [{"name": "Broth", "items": ["2 cups stock", ...]}, ...]
--   steps:       [{"text": "...", "substeps": ["...", ...]}, ...]
CREATE TABLE recipes (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    slug         text NOT NULL UNIQUE,
    title        text NOT NULL,
    tagline      text NOT NULL DEFAULT '',
    author       text NOT NULL DEFAULT '',
    servings     integer NOT NULL DEFAULT 0,
    prep_minutes integer NOT NULL DEFAULT 0,
    cook_minutes integer NOT NULL DEFAULT 0,
    calories     integer NOT NULL DEFAULT 0,
    source_url   text NOT NULL DEFAULT '',
    notes        text NOT NULL DEFAULT '',
    status       text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    tags         text[] NOT NULL DEFAULT '{}',
    ingredients  jsonb NOT NULL DEFAULT '[]',
    steps        jsonb NOT NULL DEFAULT '[]',
    image_key    text NOT NULL DEFAULT '',
    image_thumb_key text NOT NULL DEFAULT '',
    image_type   text NOT NULL DEFAULT '',
    search       tsvector NOT NULL DEFAULT ''::tsvector,
    created_by   uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz
);

CREATE INDEX idx_recipes_search ON recipes USING gin (search);
CREATE INDEX idx_recipes_tags ON recipes USING gin (tags);
CREATE INDEX idx_recipes_updated ON recipes (updated_at DESC) WHERE deleted_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION recipes_search_update() RETURNS trigger AS $$
BEGIN
    NEW.search :=
        setweight(to_tsvector('english', coalesce(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', array_to_string(NEW.tags, ' ')), 'A') ||
        setweight(to_tsvector('english', coalesce(NEW.tagline, '')), 'B') ||
        setweight(to_tsvector('english',
            coalesce(jsonb_path_query_array(NEW.ingredients, '$[*].items[*]')::text, '')), 'C') ||
        setweight(to_tsvector('english', coalesce(NEW.notes, '')), 'D');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER recipes_search_trigger
    BEFORE INSERT OR UPDATE ON recipes
    FOR EACH ROW EXECUTE FUNCTION recipes_search_update();

-- Every change to a recipe (web or agent) appends a full snapshot, so a bad
-- edit is one restore away.
CREATE TABLE recipe_revisions (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    recipe_id  uuid NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    revision   integer NOT NULL,
    action     text NOT NULL,        -- create | update | delete | restore
    actor      text NOT NULL,        -- "web" or "key:<api key name>"
    user_id    uuid REFERENCES users(id) ON DELETE SET NULL,
    snapshot   jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (recipe_id, revision)
);

-- Share links: unguessable token, view-only, revocable, optional expiry.
CREATE TABLE recipe_shares (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    recipe_id  uuid NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    token      text NOT NULL UNIQUE,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    revoked_at timestamptz
);

CREATE INDEX idx_recipe_shares_recipe ON recipe_shares (recipe_id);

-- OAuth 2.1 authorization server (for ChatGPT / claude.ai connectors).
-- Clients are public (PKCE, no secret) and self-register via RFC 7591.
CREATE TABLE oauth_clients (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    name          text NOT NULL,
    redirect_uris text[] NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- Short-lived, single-use authorization codes (stored hashed).
CREATE TABLE oauth_codes (
    code_hash      text PRIMARY KEY,
    client_id      uuid NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri   text NOT NULL,
    code_challenge text NOT NULL,
    scopes         text[] NOT NULL,
    expires_at     timestamptz NOT NULL
);

-- A grant is one user's connection to one client; revoking it (deleting the
-- row) kills all of its tokens.
CREATE TABLE oauth_grants (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    client_id    uuid NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scopes       text[] NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz
);
CREATE INDEX oauth_grants_user_idx ON oauth_grants (user_id);

CREATE TABLE oauth_tokens (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    grant_id   uuid NOT NULL REFERENCES oauth_grants(id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('access', 'refresh')),
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX oauth_tokens_grant_idx ON oauth_tokens (grant_id);

-- +goose Down
DROP TABLE IF EXISTS oauth_tokens;
DROP TABLE IF EXISTS oauth_grants;
DROP TABLE IF EXISTS oauth_codes;
DROP TABLE IF EXISTS oauth_clients;
DROP TABLE IF EXISTS recipe_shares;
DROP TABLE IF EXISTS recipe_revisions;
DROP TRIGGER IF EXISTS recipes_search_trigger ON recipes;
DROP FUNCTION IF EXISTS recipes_search_update();
DROP TABLE IF EXISTS recipes;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS invites;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
