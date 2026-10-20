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
	// CookieName is the session cookie's name; huma handlers reference it
	// directly via the `cookie:"..."` input tag.
	CookieName = "cookira_session"
	sessionTTL = 30 * 24 * time.Hour
)

// SecureCookies marks session cookies Secure; main sets it when the public
// URL is https.
var SecureCookies bool

var ErrInvalidSession = errors.New("invalid or expired session")

// Manager creates, validates and destroys DB-backed sessions.
type Manager struct {
	q *db.Queries
}

func NewManager(q *db.Queries) *Manager {
	return &Manager{q: q}
}

// CreateSession creates a new session for userID and returns the Set-Cookie
// header value for it (huma output structs set this via a
// `header:"Set-Cookie"` field).
func (m *Manager) CreateSession(ctx context.Context, userID pgtype.UUID) (http.Cookie, error) {
	sess, err := m.q.CreateSession(ctx, db.CreateSessionParams{
		UserID:    userID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(sessionTTL), Valid: true},
	})
	if err != nil {
		return http.Cookie{}, err
	}
	return NewCookie(sess.ID.String(), sess.ExpiresAt.Time), nil
}

// DeleteSession removes the session with the given (string) ID. Malformed
// IDs are treated as already-gone rather than an error.
func (m *Manager) DeleteSession(ctx context.Context, idStr string) error {
	if idStr == "" {
		return nil
	}
	var id pgtype.UUID
	if err := id.Scan(idStr); err != nil {
		return nil
	}
	return m.q.DeleteSession(ctx, id)
}

// IdentityFromRequest resolves the caller from either a Bearer API key
// (checked first — agents/scripts) or the session cookie (browser).
// Used by the RequireAuth middleware and the huma-level requireAuth.
func (m *Manager) IdentityFromRequest(ctx context.Context, r *http.Request) (Identity, error) {
	if key := bearerToken(r); key != "" {
		if isOAuthAccessToken(key) {
			return m.identityFromOAuthToken(ctx, key)
		}
		return m.identityFromAPIKey(ctx, key)
	}
	return m.identityFromCookie(ctx, r)
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}

func (m *Manager) identityFromAPIKey(ctx context.Context, plaintext string) (Identity, error) {
	key, err := m.q.GetAPIKeyByHash(ctx, HashAPIKey(plaintext))
	if err != nil {
		return Identity{}, ErrInvalidSession
	}
	go func() {
		_ = m.q.UpdateAPIKeyLastUsed(context.Background(), key.ID)
	}()
	return Identity{UserID: key.UserID.String(), ViaAPIKey: true, Write: ScopesCanWrite(key.Scopes), KeyName: key.Name}, nil
}

func (m *Manager) identityFromCookie(ctx context.Context, r *http.Request) (Identity, error) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return Identity{}, ErrInvalidSession
	}
	var id pgtype.UUID
	if err := id.Scan(c.Value); err != nil {
		return Identity{}, ErrInvalidSession
	}
	sess, err := m.q.GetSession(ctx, id)
	if err != nil {
		return Identity{}, ErrInvalidSession
	}
	if !sess.ExpiresAt.Valid || sess.ExpiresAt.Time.Before(time.Now()) {
		return Identity{}, ErrInvalidSession
	}
	return Identity{UserID: sess.UserID.String(), Write: true}, nil
}

// NewCookie builds the Set-Cookie value for a live session.
func NewCookie(value string, expires time.Time) http.Cookie {
	return http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   SecureCookies,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	}
}

// ClearedCookie builds a Set-Cookie value that deletes the session cookie.
func ClearedCookie() http.Cookie {
	return http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
}
