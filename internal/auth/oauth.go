package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"
)

// OAuth tokens are random like API keys, so they're stored with the same
// fast hash (HashAPIKey) and told apart from API keys by prefix.
const (
	AccessTokenPrefix  = "cat_"
	RefreshTokenPrefix = "crt_"
	OAuthKeyPrefix     = "oauth:"
)

// GenerateOAuthToken returns a new random token with the given prefix and
// its stored hash.
func GenerateOAuthToken(prefix string) (plaintext, hash string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	plaintext = prefix + base64.RawURLEncoding.EncodeToString(b)
	return plaintext, HashAPIKey(plaintext)
}

// identityFromOAuthToken resolves an OAuth access token. The identity is
// marked ViaAPIKey so session-only operations (key management) stay closed
// to connected apps, and KeyName is "oauth:<client name>" so recipe
// revisions show which connector made a change.
func (m *Manager) identityFromOAuthToken(ctx context.Context, plaintext string) (Identity, error) {
	t, err := m.q.GetOAuthTokenByHash(ctx, HashAPIKey(plaintext))
	if err != nil || t.Kind != "access" || !t.ExpiresAt.Valid || t.ExpiresAt.Time.Before(time.Now()) {
		return Identity{}, ErrInvalidSession
	}
	go func() {
		_ = m.q.TouchOAuthGrant(context.Background(), t.GrantID)
	}()
	return Identity{
		UserID:    t.UserID.String(),
		ViaAPIKey: true,
		Write:     ScopesCanWrite(t.Scopes),
		KeyName:   OAuthKeyPrefix + t.ClientName,
	}, nil
}

func isOAuthAccessToken(s string) bool { return strings.HasPrefix(s, AccessTokenPrefix) }
