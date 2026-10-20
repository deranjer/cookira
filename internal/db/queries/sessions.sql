-- name: CreateSession :one
INSERT INTO sessions (user_id, expires_at)
VALUES ($1, $2)
RETURNING *;

-- name: GetSession :one
SELECT * FROM sessions WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;
