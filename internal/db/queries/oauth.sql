-- name: CreateOAuthClient :one
INSERT INTO oauth_clients (name, redirect_uris)
VALUES ($1, $2)
RETURNING *;

-- name: GetOAuthClient :one
SELECT * FROM oauth_clients WHERE id = $1;

-- Drops clients that registered but never completed a connection.
-- name: DeleteStaleOAuthClients :exec
DELETE FROM oauth_clients c
WHERE c.created_at < now() - interval '1 day'
  AND NOT EXISTS (SELECT 1 FROM oauth_grants g WHERE g.client_id = c.id);

-- name: CountOAuthClients :one
SELECT count(*) FROM oauth_clients;

-- name: CreateOAuthCode :exec
INSERT INTO oauth_codes (code_hash, client_id, user_id, redirect_uri, code_challenge, scopes, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- Authorization codes are single-use: consuming one deletes it.
-- name: ConsumeOAuthCode :one
DELETE FROM oauth_codes WHERE code_hash = $1 RETURNING *;

-- name: DeleteExpiredOAuthCodes :exec
DELETE FROM oauth_codes WHERE expires_at < now();

-- name: DeleteOAuthGrantsForClientUser :exec
DELETE FROM oauth_grants WHERE client_id = $1 AND user_id = $2;

-- name: CreateOAuthGrant :one
INSERT INTO oauth_grants (client_id, user_id, scopes)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateOAuthToken :exec
INSERT INTO oauth_tokens (grant_id, kind, token_hash, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetOAuthTokenByHash :one
SELECT t.id AS token_id, t.kind, t.expires_at, g.id AS grant_id, g.client_id, g.user_id, g.scopes, c.name AS client_name
FROM oauth_tokens t
JOIN oauth_grants g ON g.id = t.grant_id
JOIN oauth_clients c ON c.id = g.client_id
WHERE t.token_hash = $1;

-- name: DeleteOAuthToken :exec
DELETE FROM oauth_tokens WHERE id = $1;

-- name: DeleteOAuthGrant :execrows
DELETE FROM oauth_grants WHERE id = $1 AND user_id = $2;

-- name: DeleteOAuthGrantByID :exec
DELETE FROM oauth_grants WHERE id = $1;

-- name: DeleteExpiredOAuthTokens :exec
DELETE FROM oauth_tokens WHERE expires_at < now();

-- name: TouchOAuthGrant :exec
UPDATE oauth_grants SET last_used_at = now() WHERE id = $1;

-- name: ListOAuthGrantsByUser :many
SELECT g.id, g.scopes, g.created_at, g.last_used_at, c.name AS client_name
FROM oauth_grants g
JOIN oauth_clients c ON c.id = g.client_id
WHERE g.user_id = $1
ORDER BY g.created_at DESC;
