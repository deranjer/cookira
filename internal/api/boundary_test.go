package api

import (
	"net/http"
	"strings"
	"testing"
)

// Credentials must not cross surfaces: people's logins work on the REST API,
// OAuth tokens work on /mcp, and nothing else (API keys no longer exist).
func TestCredentialBoundaries(t *testing.T) {
	e := newOAuthEnv(t)
	clientID := e.registerClient("Boundary")
	code := e.authorize(clientID, true, true, "").Query().Get("code")
	access := e.exchange(clientID, code)["access_token"].(string)

	// The OAuth token works on /mcp...
	if rec := e.mcp(access, mcpInitialize); rec.Code != http.StatusOK {
		t.Fatalf("oauth token on /mcp = %d: %s", rec.Code, rec.Body.String())
	}

	// ...but not on the REST API, nor to approve another connection.
	for _, path := range []string{"/api/v1/auth/me", "/api/v1/recipes", "/api/v1/oauth/connections"} {
		if rec := e.do("GET", path, "", "", bearer(access)); rec.Code != http.StatusUnauthorized {
			t.Errorf("oauth token on GET %s = %d, want 401", path, rec.Code)
		}
	}
	rec := e.do("POST", "/oauth/authorize/decision", "application/json", e.authorizeBody(clientID, func(map[string]any) {}), bearer(access))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("oauth token approving a connection = %d, want 401", rec.Code)
	}

	// A person's session cookie or login JWT can't be replayed against /mcp.
	cookieRec := e.do("POST", "/mcp", "application/json", mcpInitialize, func(r *http.Request) {
		r.Header.Set("Accept", "application/json, text/event-stream")
		e.withSession(r)
	})
	if cookieRec.Code != http.StatusUnauthorized {
		t.Errorf("session cookie on /mcp = %d, want 401", cookieRec.Code)
	}
	if rec := e.mcp(e.cookie.Value, mcpInitialize); rec.Code != http.StatusUnauthorized {
		t.Errorf("login JWT on /mcp = %d, want 401", rec.Code)
	}
	if !strings.Contains(cookieRec.Header().Get("WWW-Authenticate"), "resource_metadata") {
		t.Errorf("401 on /mcp lacks the OAuth challenge: %v", cookieRec.Header())
	}

	// API keys are gone: the endpoints don't exist and a ck_ token is just invalid.
	for _, method := range []string{"GET", "POST"} {
		if rec := e.do(method, "/api/v1/api-keys", "application/json", "{}", e.withSession); rec.Code != http.StatusNotFound {
			t.Errorf("%s /api/v1/api-keys = %d, want 404", method, rec.Code)
		}
	}
	if rec := e.mcp("ck_0123456789abcdef", mcpInitialize); rec.Code != http.StatusUnauthorized {
		t.Errorf("ck_ token on /mcp = %d, want 401", rec.Code)
	}
}

// The unauthenticated OAuth endpoints are rate limited per peer address.
func TestOAuthRegisterRateLimit(t *testing.T) {
	e := newOAuthEnv(t)
	body := `{"client_name":"Spam","redirect_uris":["https://chatgpt.example/callback"]}`
	var limited bool
	for i := 0; i < 12; i++ {
		rec := e.do("POST", "/oauth/register", "application/json", body, nil)
		if rec.Code == http.StatusTooManyRequests {
			limited = true
			if rec.Header().Get("Retry-After") == "" {
				t.Error("429 without Retry-After")
			}
			break
		}
	}
	if !limited {
		t.Error("registration was never rate limited")
	}
}

// GET /oauth/authorize is the consent page, served by the web app's catch-all.
// A wildcard OPTIONS route once made it answer 405 instead of falling through.
func TestConsentPageIsNotMethodNotAllowed(t *testing.T) {
	e := newOAuthEnv(t)
	if rec := e.do("GET", "/oauth/authorize", "", "", nil); rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("GET /oauth/authorize = 405; it must fall through to the web app")
	}
	// Preflights for the real endpoints still work.
	if rec := e.do("OPTIONS", "/oauth/token", "", "", nil); rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS /oauth/token = %d, want 204", rec.Code)
	}
}
