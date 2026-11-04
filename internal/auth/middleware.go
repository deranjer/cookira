package auth

import (
	"net/http"
)

func unauthorized(w http.ResponseWriter, challenge string) {
	w.Header().Set("Content-Type", "application/json")
	if challenge != "" {
		w.Header().Set("WWW-Authenticate", challenge)
	}
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"status":401,"title":"Unauthorized","detail":"login required"}`))
}

// RequireAuth validates a person's login (session cookie or login JWT) and
// injects the resolved Identity into the request context, or responds 401.
// OAuth access tokens are not accepted.
func RequireAuth(m *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := m.IdentityFromRequest(r.Context(), r)
			if err != nil {
				unauthorized(w, "")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

// RequireOAuth guards /mcp: only an OAuth access token is accepted. A 401
// carries a WWW-Authenticate challenge pointing at resourceMetadataURL
// (RFC 9728), which is how MCP clients discover the authorization server.
func RequireOAuth(m *Manager, resourceMetadataURL string) func(http.Handler) http.Handler {
	challenge := `Bearer resource_metadata="` + resourceMetadataURL + `"`
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := m.IdentityFromOAuthRequest(r.Context(), r)
			if err != nil {
				unauthorized(w, challenge)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}
