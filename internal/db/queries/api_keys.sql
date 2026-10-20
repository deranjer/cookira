-- name: CreateAPIKey :one
INSERT INTO api_keys (user_id, name, key_hash, scopes)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListAPIKeysByUser :many
SELECT * FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC;

-- name: GetAPIKeyByHash :one
SELECT id, user_id, name, scopes FROM api_keys WHERE key_hash = $1;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_keys SET last_used_at = now() WHERE id = $1;

-- name: DeleteAPIKey :execrows
DELETE FROM api_keys WHERE id = $1 AND user_id = $2;
