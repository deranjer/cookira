package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/deranjer/cookira/internal/db"
)

const (
	// AccessCookieName holds the JWT access token; RefreshCookieName the
	// opaque refresh token. Browsers use cookies; other clients (mobile)
	// use bearer tokens in headers/bodies instead and never see these.
	AccessCookieName  = "cookira_token"
	RefreshCookieName = "cookira_refresh"

	// refreshCookiePath scopes the refresh cookie to the auth endpoints so
	// it isn't sent with every API request.
	refreshCookiePath = "/api/v1/auth"

	// refreshReuseGrace is how soon after a refresh token was rotated a
	// second redemption is treated as a benign race (two tabs refreshing at
	// once) instead of token theft.
	refreshReuseGrace = 30 * time.Second
)

// SecureCookies marks auth cookies Secure; main sets it when the public URL
// is https.
var SecureCookies bool

var ErrInvalidSession = errors.New("invalid or expired session")

// Tokens is a freshly issued access/refresh pair.
type Tokens struct {
	Access     string
	AccessExp  time.Time
	Refresh    string
	RefreshExp time.Time
}

// Manager issues, rotates and validates login credentials.
type Manager struct {
	q      *db.Queries
	secret []byte
}

func NewManager(q *db.Queries, secret []byte) *Manager {
	return &Manager{q: q, secret: secret}
}

// IssueTokens mints an access token and a new refresh token for userID.
func (m *Manager) IssueTokens(ctx context.Context, userID pgtype.UUID) (Tokens, error) {
	now := time.Now()
	access, accessExp, err := SignAccessToken(m.secret, userID.String(), now)
	if err != nil {
		return Tokens{}, err
	}
	refresh, hash := GenerateRefreshToken()
	refreshExp := now.Add(RefreshTokenTTL)
	if _, err := m.q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: pgtype.Timestamptz{Time: refreshExp, Valid: true},
	}); err != nil {
		return Tokens{}, err
	}
	return Tokens{Access: access, AccessExp: accessExp, Refresh: refresh, RefreshExp: refreshExp}, nil
}

// Refresh redeems a refresh token: it is revoked and a new pair issued. A
// token that was already redeemed (outside the race grace) means it leaked
// and was replayed, so every refresh token of that user is revoked.
func (m *Manager) Refresh(ctx context.Context, raw string) (pgtype.UUID, Tokens, error) {
	if raw == "" {
		return pgtype.UUID{}, Tokens{}, ErrInvalidSession
	}
	rt, err := m.q.GetRefreshTokenByHash(ctx, HashToken(raw))
	if err != nil {
		return pgtype.UUID{}, Tokens{}, ErrInvalidSession
	}
	if rt.RevokedAt.Valid {
		if time.Since(rt.RevokedAt.Time) > refreshReuseGrace {
			_ = m.q.RevokeAllRefreshTokensForUser(ctx, rt.UserID)
		}
		return pgtype.UUID{}, Tokens{}, ErrInvalidSession
	}
	if rt.ExpiresAt.Time.Before(time.Now()) {
		return pgtype.UUID{}, Tokens{}, ErrInvalidSession
	}
	n, err := m.q.RevokeRefreshToken(ctx, rt.ID)
	if err != nil {
		return pgtype.UUID{}, Tokens{}, err
	}
	if n == 0 { // lost a race with a concurrent redemption
		return pgtype.UUID{}, Tokens{}, ErrInvalidSession
	}
	tokens, err := m.IssueTokens(ctx, rt.UserID)
	return rt.UserID, tokens, err
}

// Revoke invalidates a single refresh token (logout). Unknown tokens are
// ignored.
func (m *Manager) Revoke(ctx context.Context, raw string) {
	if raw != "" {
		_ = m.q.RevokeRefreshTokenByHash(ctx, HashToken(raw))
	}
}

// RevokeAll signs a user out everywhere, optionally keeping one refresh
// token (the caller's own) alive.
func (m *Manager) RevokeAll(ctx context.Context, userID pgtype.UUID, keepRaw string) error {
	if keepRaw == "" {
		return m.q.RevokeAllRefreshTokensForUser(ctx, userID)
	}
	return m.q.RevokeOtherRefreshTokensForUser(ctx, db.RevokeOtherRefreshTokensForUserParams{
		UserID: userID, TokenHash: HashToken(keepRaw),
	})
}

// IdentityFromRequest resolves a person: a login JWT as a Bearer credential
// or, failing that, the access-token cookie (browser). OAuth access tokens
// are deliberately not accepted here; they only work on /mcp.
func (m *Manager) IdentityFromRequest(_ context.Context, r *http.Request) (Identity, error) {
	if tok := bearerToken(r); tok != "" {
		return m.identityFromJWT(tok)
	}
	c, err := r.Cookie(AccessCookieName)
	if err != nil {
		return Identity{}, ErrInvalidSession
	}
	return m.identityFromJWT(c.Value)
}

func (m *Manager) identityFromJWT(token string) (Identity, error) {
	sub, err := VerifyAccessToken(m.secret, token)
	if err != nil {
		return Identity{}, ErrInvalidSession
	}
	return Identity{UserID: sub, Write: true}, nil
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}

// IdentityFromOAuthRequest resolves a connected app: only an OAuth access
// token sent as a Bearer credential. Cookies and login JWTs are rejected so
// a browser session can never be replayed against /mcp.
func (m *Manager) IdentityFromOAuthRequest(ctx context.Context, r *http.Request) (Identity, error) {
	tok := bearerToken(r)
	if !isOAuthAccessToken(tok) {
		return Identity{}, ErrInvalidSession
	}
	return m.identityFromOAuthToken(ctx, tok)
}

// AccessCookie builds the Set-Cookie value for an access token.
func AccessCookie(token string, exp time.Time) http.Cookie {
	return http.Cookie{
		Name: AccessCookieName, Value: token, Path: "/",
		HttpOnly: true, Secure: SecureCookies, SameSite: http.SameSiteLaxMode, Expires: exp,
	}
}

// RefreshCookie builds the Set-Cookie value for a refresh token.
func RefreshCookie(token string, exp time.Time) http.Cookie {
	return http.Cookie{
		Name: RefreshCookieName, Value: token, Path: refreshCookiePath,
		HttpOnly: true, Secure: SecureCookies, SameSite: http.SameSiteLaxMode, Expires: exp,
	}
}

// ClearedCookies returns Set-Cookie values that delete both auth cookies.
func ClearedCookies() []http.Cookie {
	return []http.Cookie{
		{Name: AccessCookieName, Path: "/", HttpOnly: true, Secure: SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1},
		{Name: RefreshCookieName, Path: refreshCookiePath, HttpOnly: true, Secure: SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1},
	}
}
