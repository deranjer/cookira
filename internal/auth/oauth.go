package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"
)

// OAuth tokens are random, so they're stored with the same fast hash
// (HashToken) as the other random secrets and told apart by prefix.
const (
	AccessTokenPrefix  = "cat_"
	RefreshTokenPrefix = "crt_"
)

// GenerateOAuthToken returns a new random token with the given prefix and
// its stored hash.
func GenerateOAuthToken(prefix string) (plaintext, hash string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	plaintext = prefix + base64.RawURLEncoding.EncodeToString(b)
	return plaintext, HashToken(plaintext)
}

// identityFromOAuthToken resolves an OAuth access token. The identity is
// marked OAuth, and ClientName lands in recipe revisions as
// "oauth:<client name>" so history shows which connector made a change.
func (m *Manager) identityFromOAuthToken(ctx context.Context, plaintext string) (Identity, error) {
	t, err := m.q.GetOAuthTokenByHash(ctx, HashToken(plaintext))
	if err != nil || t.Kind != "access" || !t.ExpiresAt.Valid || t.ExpiresAt.Time.Before(time.Now()) {
		return Identity{}, ErrInvalidSession
	}
	go func() {
		_ = m.q.TouchOAuthGrant(context.Background(), t.GrantID)
	}()
	return Identity{
		UserID:     t.UserID.String(),
		OAuth:      true,
		Write:      ScopesCanWrite(t.Scopes),
		ClientName: t.ClientName,
	}, nil
}

func isOAuthAccessToken(s string) bool { return strings.HasPrefix(s, AccessTokenPrefix) }
