package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/db"
	"github.com/deranjer/cookira/internal/recipes"
	"github.com/deranjer/cookira/internal/storage"
)

const (
	testBase     = "https://cook.example"
	testRedirect = "https://chatgpt.example/callback"
	testVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
)

type oauthEnv struct {
	t      *testing.T
	srv    *Server
	userID pgtype.UUID
	cookie *http.Cookie
}

// newOAuthEnv connects to TEST_DATABASE_URL (skipping when unset) and signs
// in a fresh user. Each test uses its own user and client, so tests do not
// need an empty database.
func newOAuthEnv(t *testing.T) *oauthEnv {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	if err := db.Migrate(dbURL); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store, err := storage.NewDisk(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	q := db.New(pool)
	srv := New(pool, recipes.New(pool, q, store, testBase), testBase, []byte("test-secret-test-secret-test-secret!"))

	user, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Name: "OAuth Tester", Email: fmt.Sprintf("oauth-%d@example.com", time.Now().UnixNano()), Role: "member",
	})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := srv.mgr.IssueTokens(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	c := auth.AccessCookie(tokens.Access, tokens.AccessExp)
	return &oauthEnv{t: t, srv: srv, userID: user.ID, cookie: &c}
}

func (e *oauthEnv) do(method, path, contentType, body string, mod func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if mod != nil {
		mod(req)
	}
	rec := httptest.NewRecorder()
	e.srv.Router.ServeHTTP(rec, req)
	return rec
}

func (e *oauthEnv) withSession(r *http.Request) { r.AddCookie(e.cookie) }
func bearer(token string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

func (e *oauthEnv) json(rec *httptest.ResponseRecorder, wantStatus int) map[string]any {
	e.t.Helper()
	if rec.Code != wantStatus {
		e.t.Fatalf("status = %d, want %d: %s", rec.Code, wantStatus, rec.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		e.t.Fatalf("bad JSON %q: %v", rec.Body.String(), err)
	}
	return m
}

func (e *oauthEnv) registerClient(name string) string {
	body := fmt.Sprintf(`{"client_name":%q,"redirect_uris":[%q]}`, name, testRedirect)
	m := e.json(e.do("POST", "/oauth/register", "application/json", body, nil), http.StatusCreated)
	return m["client_id"].(string)
}

func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (e *oauthEnv) authorizeBody(clientID string, mod func(map[string]any)) string {
	m := map[string]any{
		"clientId": clientID, "redirectUri": testRedirect, "responseType": "code",
		"codeChallenge": challenge(testVerifier), "codeChallengeMethod": "S256",
		"state": "xyz", "resource": testBase + "/mcp", "approve": true,
	}
	mod(m)
	b, _ := json.Marshal(m)
	return string(b)
}

// authorize drives the consent decision and returns the redirect URL.
func (e *oauthEnv) authorize(clientID string, approve, write bool, scope string) *url.URL {
	body := e.authorizeBody(clientID, func(m map[string]any) {
		m["approve"], m["write"], m["scope"] = approve, write, scope
	})
	m := e.json(e.do("POST", "/oauth/authorize/decision", "application/json", body, e.withSession), http.StatusOK)
	u, err := url.Parse(m["redirectUrl"].(string))
	if err != nil {
		e.t.Fatal(err)
	}
	if got := u.Query().Get("state"); got != "xyz" {
		e.t.Errorf("state = %q", got)
	}
	return u
}

func (e *oauthEnv) tokenRequest(form url.Values) *httptest.ResponseRecorder {
	return e.do("POST", "/oauth/token", "application/x-www-form-urlencoded", form.Encode(), nil)
}

func codeForm(clientID, code, verifier string) url.Values {
	return url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {testRedirect},
		"client_id": {clientID}, "code_verifier": {verifier},
	}
}

func (e *oauthEnv) exchange(clientID, code string) map[string]any {
	return e.json(e.tokenRequest(codeForm(clientID, code, testVerifier)), http.StatusOK)
}

func (e *oauthEnv) mcp(token, payload string) *httptest.ResponseRecorder {
	return e.do("POST", "/mcp", "application/json", payload, func(r *http.Request) {
		r.Header.Set("Accept", "application/json, text/event-stream")
		if token != "" {
			bearer(token)(r)
		}
	})
}

const mcpInitialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`

func mcpCreate(title string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"create_recipe","arguments":{"title":%q}}}`, title)
}

func TestOAuthDiscovery(t *testing.T) {
	e := newOAuthEnv(t)

	prm := e.json(e.do("GET", "/.well-known/oauth-protected-resource", "", "", nil), http.StatusOK)
	if prm["resource"] != testBase+"/mcp" {
		t.Errorf("resource = %v", prm["resource"])
	}
	e.json(e.do("GET", "/.well-known/oauth-protected-resource/mcp", "", "", nil), http.StatusOK)

	as := e.json(e.do("GET", "/.well-known/oauth-authorization-server", "", "", nil), http.StatusOK)
	if as["issuer"] != testBase || as["token_endpoint"] != testBase+"/oauth/token" || as["registration_endpoint"] != testBase+"/oauth/register" {
		t.Errorf("metadata = %v", as)
	}

	rec := e.mcp("", `{}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	want := `Bearer resource_metadata="` + testBase + `/.well-known/oauth-protected-resource/mcp"`
	if got := rec.Header().Get("WWW-Authenticate"); got != want {
		t.Errorf("WWW-Authenticate = %q, want %q", got, want)
	}
}

func TestOAuthFlowReadWrite(t *testing.T) {
	e := newOAuthEnv(t)
	clientID := e.registerClient("ChatGPT")

	// The consent screen's info endpoint needs a session and shows the client.
	infoPath := "/oauth/authorize/info?client_id=" + clientID + "&redirect_uri=" + url.QueryEscape(testRedirect) +
		"&response_type=code&code_challenge=" + challenge(testVerifier) + "&code_challenge_method=S256"
	if rec := e.do("GET", infoPath, "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("info without session = %d", rec.Code)
	}
	info := e.json(e.do("GET", infoPath, "", "", e.withSession), http.StatusOK)
	if info["clientName"] != "ChatGPT" || info["canWrite"] != true {
		t.Errorf("info = %v", info)
	}

	redirect := e.authorize(clientID, true, true, "read write offline_access")
	code := redirect.Query().Get("code")
	if code == "" || redirect.Query().Get("iss") != testBase {
		t.Fatalf("redirect = %v", redirect)
	}

	// A wrong verifier fails, and the code is burned either way.
	if m := e.json(e.tokenRequest(codeForm(clientID, code, strings.Repeat("a", 50))), http.StatusBadRequest); m["error"] != "invalid_grant" {
		t.Errorf("bad verifier: %v", m)
	}
	if rec := e.tokenRequest(codeForm(clientID, code, testVerifier)); rec.Code != http.StatusBadRequest {
		t.Errorf("code reuse = %d", rec.Code)
	}

	code = e.authorize(clientID, true, true, "read write offline_access").Query().Get("code")
	tok := e.exchange(clientID, code)
	access, refresh := tok["access_token"].(string), tok["refresh_token"].(string)
	if !strings.HasPrefix(access, auth.AccessTokenPrefix) || refresh == "" || tok["scope"] != "read write" {
		t.Fatalf("token response = %v", tok)
	}

	// The access token works on /mcp and acts as the user with write access.
	if rec := e.mcp(access, mcpInitialize); rec.Code != http.StatusOK {
		t.Fatalf("mcp initialize = %d: %s", rec.Code, rec.Body.String())
	}
	title := fmt.Sprintf("OAuth Soup %d", time.Now().UnixNano())
	rec := e.mcp(access, mcpCreate(title))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"isError":true`) {
		t.Fatalf("create with write token: %d %s", rec.Code, rec.Body.String())
	}
	// Revisions record the connector as the actor.
	items, _, err := e.srv.svc.List(context.Background(), recipes.ListParams{Query: title, Limit: 5})
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %v %v", items, err)
	}
	revs, err := e.srv.svc.Revisions(context.Background(), items[0].ID)
	if err != nil || len(revs) == 0 || revs[0].Actor != "oauth:ChatGPT" {
		t.Errorf("history = %+v, err %v", revs, err)
	}

	// Refresh rotates: the old refresh token dies, the new pair works.
	refreshForm := func(rt string) url.Values {
		return url.Values{"grant_type": {"refresh_token"}, "refresh_token": {rt}, "client_id": {clientID}}
	}
	refreshed := e.json(e.tokenRequest(refreshForm(refresh)), http.StatusOK)
	if refreshed["refresh_token"] == refresh {
		t.Error("refresh token was not rotated")
	}
	if rec := e.tokenRequest(refreshForm(refresh)); rec.Code != http.StatusBadRequest {
		t.Errorf("reused refresh token = %d", rec.Code)
	}
	newAccess := refreshed["access_token"].(string)
	if rec := e.mcp(newAccess, mcpInitialize); rec.Code != http.StatusOK {
		t.Fatalf("refreshed token rejected: %d", rec.Code)
	}

	// Connections are listed in Settings; a connected app cannot manage
	// them, and deleting one revokes its tokens.
	const listPath = "/api/v1/oauth/connections"
	var conns []map[string]any
	rec = e.do("GET", listPath, "", "", e.withSession)
	if err := json.Unmarshal(rec.Body.Bytes(), &conns); err != nil || len(conns) != 1 || conns[0]["clientName"] != "ChatGPT" {
		t.Fatalf("connections = %s", rec.Body.String())
	}
	delPath := listPath + "/" + conns[0]["id"].(string)
	if rec := e.do("DELETE", delPath, "", "", bearer(newAccess)); rec.Code != http.StatusUnauthorized {
		t.Errorf("delete via oauth token = %d", rec.Code)
	}
	if rec := e.do("DELETE", delPath, "", "", e.withSession); rec.Code != http.StatusOK {
		t.Fatalf("delete = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := e.mcp(newAccess, mcpInitialize); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked access token = %d", rec.Code)
	}
	if rec := e.tokenRequest(refreshForm(refreshed["refresh_token"].(string))); rec.Code != http.StatusBadRequest {
		t.Errorf("revoked refresh token = %d", rec.Code)
	}
}

func TestOAuthReadOnlyAndDenial(t *testing.T) {
	e := newOAuthEnv(t)
	clientID := e.registerClient("Reader")

	code := e.authorize(clientID, true, false, "").Query().Get("code")
	access := e.exchange(clientID, code)["access_token"].(string)
	rec := e.mcp(access, mcpCreate(fmt.Sprintf("Should not exist %d", time.Now().UnixNano())))
	if !strings.Contains(rec.Body.String(), `"isError":true`) {
		t.Errorf("read-only token created a recipe: %s", rec.Body.String())
	}

	// A client that only asked for "read" cannot be upgraded by the form.
	code = e.authorize(clientID, true, true, "read").Query().Get("code")
	if tok := e.exchange(clientID, code); tok["scope"] != "read" {
		t.Errorf("scope = %v, want read", tok["scope"])
	}

	// Denial redirects with access_denied and no code.
	redirect := e.authorize(clientID, false, false, "")
	if redirect.Query().Get("code") != "" || redirect.Query().Get("error") != "access_denied" {
		t.Errorf("denied redirect = %v", redirect)
	}
}

func TestOAuthRejectsBadRequests(t *testing.T) {
	e := newOAuthEnv(t)
	clientID := e.registerClient("Strict")

	if rec := e.do("POST", "/oauth/register", "application/json", `{"redirect_uris":["http://evil.example/cb"]}`, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("register with insecure redirect = %d", rec.Code)
	}
	cases := map[string]func(map[string]any){
		"wrong redirect": func(m map[string]any) { m["redirectUri"] = "https://attacker.example/cb" },
		"no PKCE":        func(m map[string]any) { m["codeChallenge"] = "" },
		"plain PKCE":     func(m map[string]any) { m["codeChallengeMethod"] = "plain" },
		"wrong resource": func(m map[string]any) { m["resource"] = "https://other.example/mcp" },
		"unknown client": func(m map[string]any) { m["clientId"] = "00000000-0000-0000-0000-000000000000" },
	}
	for name, mod := range cases {
		rec := e.do("POST", "/oauth/authorize/decision", "application/json", e.authorizeBody(clientID, mod), e.withSession)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, rec.Code)
		}
	}
}
