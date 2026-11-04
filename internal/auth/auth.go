// Package auth implements authentication: password hashing, login JWTs with
// rotating refresh tokens, OAuth access tokens, and the middleware that
// guards the API.
//
// People sign in with email and password (httpOnly cookies for the web app,
// bearer JWTs for other clients). Agents connect to /mcp only through OAuth
// (see internal/oauth). The two never mix: OAuth tokens are rejected by the
// REST API and login credentials are rejected by /mcp.
package auth

import "context"

type contextKey string

const identityKey contextKey = "identity"

// Identity is the authenticated caller. People signing in are always full
// read-write; an OAuth connection carries the scope its user approved.
type Identity struct {
	UserID     string
	OAuth      bool   // true for a connected app (MCP), false for a person
	Write      bool   // OAuth: whether "write" was approved
	ClientName string // set when OAuth; recorded in recipe revisions
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

// Actor is a short human-readable label for who made a change, recorded in
// recipe revisions: "oauth:<client name>" for connected apps, "web" for
// people.
func Actor(ctx context.Context) string {
	id, ok := IdentityFromContext(ctx)
	switch {
	case !ok:
		return "system"
	case id.OAuth:
		return "oauth:" + id.ClientName
	default:
		return "web"
	}
}
