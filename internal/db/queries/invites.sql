-- name: CreateInvite :one
INSERT INTO invites (token_hash, email, role, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListInvites :many
SELECT * FROM invites WHERE used_at IS NULL ORDER BY created_at DESC;

-- name: GetInviteByHash :one
SELECT * FROM invites WHERE token_hash = $1;

-- ClaimInvite atomically marks a usable invite as used; no row means it was
-- unknown, expired or already redeemed.
-- name: ClaimInvite :one
UPDATE invites SET used_at = now()
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
RETURNING *;

-- name: SetInviteUsedBy :exec
UPDATE invites SET used_by = $2 WHERE id = $1;

-- name: DeleteInvite :execrows
DELETE FROM invites WHERE id = $1 AND used_at IS NULL;
