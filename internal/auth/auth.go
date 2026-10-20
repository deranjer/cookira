// Package auth implements session-cookie and API-key authentication:
// password hashing, DB-backed sessions and keys, and the chi middleware
// that guards the API. Callers authenticate with email/password (browser),
// Bearer API keys (agents/scripts) or OAuth access tokens (see internal/oauth).
package auth

import "context"

type contextKey string

const identityKey contextKey = "identity"

// Identity is the authenticated caller. Sessions (browser login) are
// always full read-write; API keys carry their own read-only/read-write
// scope.
type Identity struct {
	UserID    string
	ViaAPIKey bool
	Write     bool
	KeyName   string // set when ViaAPIKey; recorded in recipe revisions
}

// IdentityFromContext returns the authenticated caller, if any.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// UserID is a convenience wrapper over IdentityFromContext for call sites
// that only care who the caller is, not how they authenticated.
func UserID(ctx context.Context) (string, bool) {
	id, ok := IdentityFromContext(ctx)
	if !ok {
		return "", false
	}
	return id.UserID, true
}

// CanWrite reports whether the caller may perform mutating operations.
func CanWrite(ctx context.Context) bool {
	id, ok := IdentityFromContext(ctx)
	return ok && id.Write
}

// ViaAPIKey reports whether the caller authenticated with an API key
// rather than a browser session.
func ViaAPIKey(ctx context.Context) bool {
	id, ok := IdentityFromContext(ctx)
	return ok && id.ViaAPIKey
}

// Actor is a short human-readable label for who made a change, recorded in
// recipe revisions: "key:<name>" for API keys, "web" for browser sessions.
func Actor(ctx context.Context) string {
	id, ok := IdentityFromContext(ctx)
	switch {
	case !ok:
		return "system"
	case id.ViaAPIKey:
		return "key:" + id.KeyName
	default:
		return "web"
	}
}
