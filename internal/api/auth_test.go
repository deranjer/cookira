package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/db"
)

// authEnv drives the real router against TEST_DATABASE_URL. It reuses the
// OAuth tests' environment (skips when unset); every test makes its own
// users, so the database need not be empty.
type authEnv struct{ *oauthEnv }

func newAuthEnv(t *testing.T) *authEnv { return &authEnv{newOAuthEnv(t)} }

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@example.com", prefix, time.Now().UnixNano())
}

func (e *authEnv) post(path string, body any, mod func(*http.Request)) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	return e.do(http.MethodPost, path, "application/json", string(b), mod)
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, want int) map[string]any {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d: %s", rec.Code, want, rec.Body.String())
	}
	var m map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &m)
	}
	return m
}

func cookies(rec *httptest.ResponseRecorder) map[string]*http.Cookie {
	out := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		out[c.Name] = c
	}
	return out
}

// makeUser creates a user with a known password and returns its email.
func (e *authEnv) makeUser(role string) string {
	e.t.Helper()
	email := uniqueEmail("u")
	adminCookie := e.adminCookie()
	rec := e.post("/api/v1/users", map[string]any{"name": "T", "email": email, "password": "password123", "role": role},
		func(r *http.Request) { r.AddCookie(adminCookie) })
	decode(e.t, rec, 200)
	return email
}

// adminCookie promotes the env's user to admin and returns its access cookie.
func (e *authEnv) adminCookie() *http.Cookie {
	e.t.Helper()
	_, err := e.srv.q.UpdateUserProfile(e.t.Context(), updateProfile(e.userID, "Admin", "admin"))
	if err != nil {
		e.t.Fatal(err)
	}
	return e.cookie
}

func TestLoginRefreshRotationAndReuse(t *testing.T) {
	e := newAuthEnv(t)
	email := e.makeUser("member")

	// Cookie-mode login sets both cookies; the refresh cookie is path-scoped.
	rec := e.post("/api/v1/auth/login", map[string]any{"email": email, "password": "password123"}, nil)
	decode(t, rec, 200)
	cs := cookies(rec)
	if cs["cookira_token"] == nil || cs["cookira_refresh"] == nil {
		t.Fatalf("missing cookies: %v", rec.Header()["Set-Cookie"])
	}
	if cs["cookira_refresh"].Path != "/api/v1/auth" || !cs["cookira_refresh"].HttpOnly {
		t.Fatalf("refresh cookie not scoped/httpOnly: %+v", cs["cookira_refresh"])
	}

	// The access cookie authenticates.
	me := e.do(http.MethodGet, "/api/v1/auth/me", "", "", func(r *http.Request) { r.AddCookie(cs["cookira_token"]) })
	if got := decode(t, me, 200)["email"]; got != email {
		t.Fatalf("me email = %v", got)
	}

	// Refresh rotates: new refresh token works, old one is dead.
	rec2 := e.post("/api/v1/auth/refresh", map[string]any{}, func(r *http.Request) { r.AddCookie(cs["cookira_refresh"]) })
	decode(t, rec2, 200)
	cs2 := cookies(rec2)
	if cs2["cookira_refresh"] == nil || cs2["cookira_refresh"].Value == cs["cookira_refresh"].Value {
		t.Fatal("refresh token was not rotated")
	}
	// Replaying the old token within the grace window is rejected but harmless.
	decode(t, e.post("/api/v1/auth/refresh", map[string]any{}, func(r *http.Request) { r.AddCookie(cs["cookira_refresh"]) }), 401)
	decode(t, e.post("/api/v1/auth/refresh", map[string]any{}, func(r *http.Request) { r.AddCookie(cs2["cookira_refresh"]) }), 200)
}

func TestReuseOutsideGraceRevokesAll(t *testing.T) {
	e := newAuthEnv(t)
	email := e.makeUser("member")
	login := func() map[string]any {
		return decode(t, e.post("/api/v1/auth/login", map[string]any{"email": email, "password": "password123", "tokens": true}, nil), 200)
	}
	tok := login()["tokens"].(map[string]any)["refreshToken"].(string)
	other := login()["tokens"].(map[string]any)["refreshToken"].(string)

	// Redeem once, then age the revocation past the grace window and replay.
	next := decode(t, e.post("/api/v1/auth/refresh", map[string]any{"refreshToken": tok}, nil), 200)["tokens"].(map[string]any)["refreshToken"].(string)
	if _, err := e.srv.pool.Exec(t.Context(), `UPDATE refresh_tokens SET revoked_at = now() - interval '5 minutes' WHERE token_hash = $1`, hashForTest(tok)); err != nil {
		t.Fatal(err)
	}
	decode(t, e.post("/api/v1/auth/refresh", map[string]any{"refreshToken": tok}, nil), 401)

	// Theft response: every other refresh token of the user is now dead too.
	decode(t, e.post("/api/v1/auth/refresh", map[string]any{"refreshToken": next}, nil), 401)
	decode(t, e.post("/api/v1/auth/refresh", map[string]any{"refreshToken": other}, nil), 401)
}

func TestTokenModeBearerAndLogout(t *testing.T) {
	e := newAuthEnv(t)
	email := e.makeUser("member")
	body := decode(t, e.post("/api/v1/auth/login", map[string]any{"email": email, "password": "password123", "tokens": true}, nil), 200)
	toks := body["tokens"].(map[string]any)
	access, refresh := toks["accessToken"].(string), toks["refreshToken"].(string)

	// Token mode sets no cookies and the JWT works as a Bearer credential.
	rec := e.post("/api/v1/auth/login", map[string]any{"email": email, "password": "password123", "tokens": true}, nil)
	if len(rec.Header()["Set-Cookie"]) != 0 {
		t.Fatalf("token mode set cookies: %v", rec.Header()["Set-Cookie"])
	}
	decode(t, e.do(http.MethodGet, "/api/v1/auth/me", "", "", bearer(access)), 200)
	decode(t, e.do(http.MethodGet, "/api/v1/auth/me", "", "", bearer(access+"x")), 401)
	decode(t, e.do(http.MethodGet, "/api/v1/auth/me", "", "", nil), 401)

	// Logout revokes the refresh token.
	decode(t, e.post("/api/v1/auth/logout", map[string]any{"refreshToken": refresh}, nil), 200)
	decode(t, e.post("/api/v1/auth/refresh", map[string]any{"refreshToken": refresh}, nil), 401)
}

func TestLoginThrottle(t *testing.T) {
	e := newAuthEnv(t)
	email := e.makeUser("member")
	for i := 0; i < 10; i++ {
		decode(t, e.post("/api/v1/auth/login", map[string]any{"email": email, "password": "wrong-password"}, nil), 401)
	}
	// Locked out even with the right password.
	decode(t, e.post("/api/v1/auth/login", map[string]any{"email": email, "password": "password123"}, nil), 429)
}

func TestPasswordChangeRevokesOtherSessions(t *testing.T) {
	e := newAuthEnv(t)
	email := e.makeUser("member")
	login := func() (access, refresh string) {
		b := decode(t, e.post("/api/v1/auth/login", map[string]any{"email": email, "password": "password123", "tokens": true}, nil), 200)
		tk := b["tokens"].(map[string]any)
		return tk["accessToken"].(string), tk["refreshToken"].(string)
	}
	accessA, refreshA := login()
	_, refreshB := login()

	patch := func(body map[string]any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		return e.do(http.MethodPatch, "/api/v1/auth/me", "application/json", string(b), bearer(accessA))
	}
	decode(t, patch(map[string]any{"password": "new-password-1", "currentPassword": "nope", "refreshToken": refreshA}), 403)
	decode(t, patch(map[string]any{"password": "new-password-1", "currentPassword": "password123", "refreshToken": refreshA}), 200)

	decode(t, e.post("/api/v1/auth/refresh", map[string]any{"refreshToken": refreshB}, nil), 401) // other device signed out
	decode(t, e.post("/api/v1/auth/refresh", map[string]any{"refreshToken": refreshA}, nil), 200) // this one kept
	decode(t, e.post("/api/v1/auth/login", map[string]any{"email": email, "password": "new-password-1"}, nil), 200)
}

func TestInviteFlow(t *testing.T) {
	e := newAuthEnv(t)
	admin := e.adminCookie()
	asAdmin := func(r *http.Request) { r.AddCookie(admin) }

	// A non-admin can't create invites.
	member := e.makeUser("member")
	mb := decode(t, e.post("/api/v1/auth/login", map[string]any{"email": member, "password": "password123", "tokens": true}, nil), 200)
	memberTok := mb["tokens"].(map[string]any)["accessToken"].(string)
	decode(t, e.post("/api/v1/invites", map[string]any{}, bearer(memberTok)), 403)

	// Admin creates a link; the token appears only in that response.
	inv := decode(t, e.post("/api/v1/invites", map[string]any{"role": "member"}, asAdmin), 200)
	link, _ := inv["link"].(string)
	token := link[strings.LastIndex(link, "/")+1:]
	if !strings.HasPrefix(link, testBase+"/invite/") || token == "" {
		t.Fatalf("link = %q", link)
	}
	list := e.do(http.MethodGet, "/api/v1/invites", "", "", asAdmin)
	if strings.Contains(list.Body.String(), token) {
		t.Fatal("list leaked the invite token")
	}

	// Public lookup, then accept.
	decode(t, e.do(http.MethodGet, "/api/v1/invites/lookup/"+token, "", "", nil), 200)
	decode(t, e.do(http.MethodGet, "/api/v1/invites/lookup/not-a-token", "", "", nil), 404)

	email := uniqueEmail("invitee")
	// Too-short password is refused and does not burn the invite.
	decode(t, e.post("/api/v1/invites/accept/"+token, map[string]any{"name": "New", "email": email, "password": "short"}, nil), 422)
	// An email that's already taken is refused and does not burn it either.
	decode(t, e.post("/api/v1/invites/accept/"+token, map[string]any{"name": "New", "email": member, "password": "password123"}, nil), 409)

	rec := e.post("/api/v1/invites/accept/"+token, map[string]any{"name": "New", "email": email, "password": "password123"}, nil)
	got := decode(t, rec, 200)
	if got["role"] != "member" || cookies(rec)["cookira_token"] == nil {
		t.Fatalf("accept result: %v cookies=%v", got, rec.Header()["Set-Cookie"])
	}

	// Single use.
	decode(t, e.post("/api/v1/invites/accept/"+token, map[string]any{"name": "Again", "email": uniqueEmail("x"), "password": "password123"}, nil), 404)
	decode(t, e.do(http.MethodGet, "/api/v1/invites/lookup/"+token, "", "", nil), 404)

	// An email-locked invite only works for that address.
	locked := uniqueEmail("locked")
	inv2 := decode(t, e.post("/api/v1/invites", map[string]any{"email": locked, "role": "admin"}, asAdmin), 200)
	token2 := inv2["link"].(string)
	token2 = token2[strings.LastIndex(token2, "/")+1:]
	decode(t, e.post("/api/v1/invites/accept/"+token2, map[string]any{"name": "N", "email": uniqueEmail("other"), "password": "password123"}, nil), 422)
	got2 := decode(t, e.post("/api/v1/invites/accept/"+token2, map[string]any{"name": "N", "password": "password123"}, nil), 200)
	if got2["email"] != locked || got2["role"] != "admin" {
		t.Fatalf("locked accept = %v", got2)
	}

	// Revoking an invite kills the link.
	inv3 := decode(t, e.post("/api/v1/invites", map[string]any{}, asAdmin), 200)
	token3 := inv3["link"].(string)
	token3 = token3[strings.LastIndex(token3, "/")+1:]
	decode(t, e.do(http.MethodDelete, "/api/v1/invites/"+inv3["id"].(string), "", "", asAdmin), 200)
	decode(t, e.do(http.MethodGet, "/api/v1/invites/lookup/"+token3, "", "", nil), 404)
}

func TestAdminUserManagement(t *testing.T) {
	e := newAuthEnv(t)
	admin := e.adminCookie()
	asAdmin := func(r *http.Request) { r.AddCookie(admin) }
	email := e.makeUser("member")

	var targetID string
	users := e.do(http.MethodGet, "/api/v1/users", "", "", asAdmin)
	var list []map[string]any
	_ = json.Unmarshal(users.Body.Bytes(), &list)
	for _, u := range list {
		if u["email"] == email {
			targetID = u["id"].(string)
		}
	}
	if targetID == "" {
		t.Fatal("created user not listed")
	}

	patch := func(id string, body map[string]any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		return e.do(http.MethodPatch, "/api/v1/users/"+id, "application/json", string(b), asAdmin)
	}
	if got := decode(t, patch(targetID, map[string]any{"role": "admin", "name": "Renamed"}), 200); got["role"] != "admin" || got["name"] != "Renamed" {
		t.Fatalf("patch = %v", got)
	}

	// Admin password reset signs the user out and the new password works.
	b := decode(t, e.post("/api/v1/auth/login", map[string]any{"email": email, "password": "password123", "tokens": true}, nil), 200)
	refresh := b["tokens"].(map[string]any)["refreshToken"].(string)
	decode(t, patch(targetID, map[string]any{"password": "reset-password-1"}), 200)
	decode(t, e.post("/api/v1/auth/refresh", map[string]any{"refreshToken": refresh}, nil), 401)
	decode(t, e.post("/api/v1/auth/login", map[string]any{"email": email, "password": "reset-password-1"}, nil), 200)

	// You can't remove yourself, and removing someone else works.
	decode(t, e.do(http.MethodDelete, "/api/v1/users/"+e.userID.String(), "", "", asAdmin), 409)
	decode(t, e.do(http.MethodDelete, "/api/v1/users/"+targetID, "", "", asAdmin), 200)
	decode(t, e.do(http.MethodGet, "/api/v1/users/"+targetID, "", "", asAdmin), 405)
	decode(t, patch(targetID, map[string]any{"name": "gone"}), 404)
}

func updateProfile(id pgtype.UUID, name, role string) db.UpdateUserProfileParams {
	return db.UpdateUserProfileParams{ID: id, Name: name, Role: role}
}

func hashForTest(s string) string { return auth.HashAPIKey(s) }
