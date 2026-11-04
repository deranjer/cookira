-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens WHERE token_hash = $1;

-- RevokeRefreshToken is a compare-and-swap: only the caller that flips
-- revoked_at from NULL wins, so two racing redemptions can't both succeed.
-- name: RevokeRefreshToken :execrows
UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeRefreshTokenByHash :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: RevokeAllRefreshTokensForUser :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;

-- name: RevokeOtherRefreshTokensForUser :exec
UPDATE refresh_tokens SET revoked_at = now()
WHERE user_id = $1 AND token_hash <> $2 AND revoked_at IS NULL;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens WHERE expires_at < now() - interval '7 days';
