// Package oauth implements the OAuth 2.1 authorization server that lets
// MCP clients which cannot send a Bearer API key (ChatGPT, claude.ai
// connectors) connect to /mcp: RFC 8414 / RFC 9728 discovery, RFC 7591
// dynamic client registration, the authorization-code flow with PKCE (S256)
// and rotating refresh tokens.
//
// Users approve a connection on a consent page served by the React app,
// which reuses the browser session; the page talks to the JSON endpoints
// under /oauth/authorize/. Issued access tokens resolve to an auth.Identity
// (see auth.Manager), so the rest of the app is unaware of how a caller
// authenticated.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/db"
)

const (
	codeTTL         = 5 * time.Minute
	accessTokenTTL  = time.Hour
	refreshTokenTTL = 90 * 24 * time.Hour
	maxClients      = 1000

	ScopeRead    = "read"
	ScopeWrite   = "write"
	ScopeOffline = "offline_access"
)

type Server struct {
	pool *pgxpool.Pool
	q    *db.Queries
	mgr  *auth.Manager
	base string // public origin, no trailing slash; also the issuer

	// Per-peer-IP limits on the unauthenticated endpoints, which write to
	// the database: registration creates client rows, token/revoke hash and
	// look up secrets. Forwarding headers are ignored on purpose: clients
	// behind a reverse proxy share the proxy's budget.
	registerLimit *auth.WindowLimiter
	tokenLimit    *auth.WindowLimiter
}

func New(pool *pgxpool.Pool, mgr *auth.Manager, publicURL string) *Server {
	return &Server{
		pool: pool, q: db.New(pool), mgr: mgr, base: strings.TrimRight(publicURL, "/"),
		registerLimit: auth.NewWindowLimiter(10, time.Minute),
		tokenLimit:    auth.NewWindowLimiter(60, time.Minute),
	}
}

// limit rejects requests beyond l's per-minute budget for the peer address.
func limit(l *auth.WindowLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if !l.Allow(host) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests; try again in a minute")
			return
		}
		next(w, r)
	}
}

// ResourceMetadataURL is where /mcp's WWW-Authenticate challenge points.
func (s *Server) ResourceMetadataURL() string {
	return s.base + "/.well-known/oauth-protected-resource/mcp"
}

func (s *Server) Mount(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(cors)
		r.Get("/.well-known/oauth-protected-resource", s.protectedResource)
		r.Get("/.well-known/oauth-protected-resource/mcp", s.protectedResource)
		r.Get("/.well-known/oauth-authorization-server", s.serverMetadata)
		r.Post("/oauth/register", limit(s.registerLimit, s.register))
		r.Post("/oauth/token", limit(s.tokenLimit, s.token))
		r.Post("/oauth/revoke", limit(s.tokenLimit, s.revoke))
		// CORS preflights, listed per endpoint rather than as an "/oauth/*"
		// wildcard: a wildcard would make GET /oauth/authorize (the consent page
		// served by the web app) answer 405 instead of falling through to it.
		r.Options("/oauth/register", noContent)
		r.Options("/oauth/token", noContent)
		r.Options("/oauth/revoke", noContent)
		r.Options("/.well-known/oauth-protected-resource", noContent)
		r.Options("/.well-known/oauth-protected-resource/mcp", noContent)
		r.Options("/.well-known/oauth-authorization-server", noContent)
	})
	r.Group(func(r chi.Router) {
		// Approving a connection takes a person's login; OAuth tokens are
		// rejected by RequireAuth, so a connected app can't approve another.
		r.Use(auth.RequireAuth(s.mgr))
		r.Get("/oauth/authorize/info", s.authorizeInfo)
		r.Post("/oauth/authorize/decision", s.authorizeDecision)
	})
}

func noContent(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }

// cors lets browser-based MCP clients (e.g. the MCP inspector) use the
// public, credential-less endpoints.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, MCP-Protocol-Version")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) protectedResource(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 s.base + "/mcp",
		"authorization_servers":    []string{s.base},
		"scopes_supported":         []string{ScopeRead, ScopeWrite, ScopeOffline},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "Cookira",
	})
}

func (s *Server) serverMetadata(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         s.base,
		"authorization_endpoint":                         s.base + "/oauth/authorize",
		"token_endpoint":                                 s.base + "/oauth/token",
		"registration_endpoint":                          s.base + "/oauth/register",
		"revocation_endpoint":                            s.base + "/oauth/revoke",
		"scopes_supported":                               []string{ScopeRead, ScopeWrite, ScopeOffline},
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"authorization_response_iss_parameter_supported": true,
	})
}

// ---- dynamic client registration (RFC 7591) ----

type registerRequest struct {
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_client_metadata", "invalid JSON body")
		return
	}
	if len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > 10 {
		writeError(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uris must list 1-10 URIs")
		return
	}
	for _, u := range req.RedirectURIs {
		if err := validateRedirectURI(u); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
			return
		}
	}
	name := strings.TrimSpace(req.ClientName)
	if name == "" {
		name = "MCP client"
	}
	if runes := []rune(name); len(runes) > 100 {
		name = string(runes[:100])
	}

	ctx := r.Context()
	// Registration is open, so keep the table bounded: drop abandoned
	// clients and refuse once it is still full.
	_ = s.q.DeleteStaleOAuthClients(ctx)
	_ = s.q.DeleteExpiredOAuthCodes(ctx)
	_ = s.q.DeleteExpiredOAuthTokens(ctx)
	if n, err := s.q.CountOAuthClients(ctx); err == nil && n >= maxClients {
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "too many registered clients")
		return
	}
	c, err := s.q.CreateOAuthClient(ctx, db.CreateOAuthClientParams{Name: name, RedirectUris: req.RedirectURIs})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "could not register client")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":                  c.ID.String(),
		"client_name":                c.Name,
		"client_id_issued_at":        c.CreatedAt.Time.Unix(),
		"redirect_uris":              c.RedirectUris,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	})
}

var loopbackHosts = []string{"localhost", "127.0.0.1", "::1"}

func validateRedirectURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Fragment != "" || u.User != nil {
		return errors.New("redirect URIs must be absolute, without credentials or fragment")
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		if u.Host == "" {
			return errors.New("redirect URI needs a host")
		}
	case "http":
		if !slices.Contains(loopbackHosts, u.Hostname()) {
			return errors.New("http redirect URIs are only allowed for localhost")
		}
	case "javascript", "data", "file", "vbscript", "blob", "about", "ftp":
		return errors.New("unsupported redirect URI scheme")
	}
	return nil
}

// redirectAllowed does exact matching, except that loopback http URIs may
// differ in port (RFC 8252 section 7.3: native apps pick a free port).
func redirectAllowed(registered []string, got string) bool {
	if slices.Contains(registered, got) {
		return true
	}
	g, err := url.Parse(got)
	if err != nil || g.Scheme != "http" || !slices.Contains(loopbackHosts, g.Hostname()) {
		return false
	}
	for _, reg := range registered {
		u, err := url.Parse(reg)
		if err != nil || u.Scheme != "http" || u.Hostname() != g.Hostname() {
			continue
		}
		if u.Path == g.Path && u.RawQuery == g.RawQuery {
			return true
		}
	}
	return false
}

// ---- authorization endpoint (consent screen API) ----

type authRequest struct {
	ClientID            string `json:"clientId"`
	RedirectURI         string `json:"redirectUri"`
	ResponseType        string `json:"responseType"`
	CodeChallenge       string `json:"codeChallenge"`
	CodeChallengeMethod string `json:"codeChallengeMethod"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	Resource            string `json:"resource"`
}

type validatedRequest struct {
	client   db.OauthClient
	canWrite bool // the client did not restrict itself to read-only
}

func (s *Server) validate(ctx context.Context, a authRequest) (*validatedRequest, error) {
	var id pgtype.UUID
	if err := id.Scan(a.ClientID); err != nil || a.ClientID == "" {
		return nil, errors.New("unknown client")
	}
	client, err := s.q.GetOAuthClient(ctx, id)
	if err != nil {
		return nil, errors.New("unknown client")
	}
	if !redirectAllowed(client.RedirectUris, a.RedirectURI) {
		return nil, errors.New("redirect URI does not match the client's registration")
	}
	if a.ResponseType != "code" {
		return nil, errors.New("response_type must be code")
	}
	if a.CodeChallengeMethod != "S256" || len(a.CodeChallenge) < 43 || len(a.CodeChallenge) > 128 {
		return nil, errors.New("PKCE with code_challenge_method=S256 is required")
	}
	if a.Resource != "" && a.Resource != s.base && a.Resource != s.base+"/mcp" {
		return nil, errors.New("unknown resource")
	}
	scopes := strings.Fields(a.Scope)
	return &validatedRequest{client: client, canWrite: len(scopes) == 0 || slices.Contains(scopes, ScopeWrite)}, nil
}

func (s *Server) authorizeInfo(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	a := authRequest{
		ClientID: q.Get("client_id"), RedirectURI: q.Get("redirect_uri"), ResponseType: q.Get("response_type"),
		CodeChallenge: q.Get("code_challenge"), CodeChallengeMethod: q.Get("code_challenge_method"),
		Scope: q.Get("scope"), State: q.Get("state"), Resource: q.Get("resource"),
	}
	v, err := s.validate(r.Context(), a)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	redirectHost := ""
	if u, err := url.Parse(a.RedirectURI); err == nil {
		redirectHost = u.Host
		if redirectHost == "" {
			redirectHost = u.Scheme + "://"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"clientName":   v.client.Name,
		"redirectHost": redirectHost,
		"canWrite":     v.canWrite,
	})
}

type decisionRequest struct {
	authRequest
	Approve bool `json:"approve"`
	Write   bool `json:"write"`
}

func (s *Server) authorizeDecision(w http.ResponseWriter, r *http.Request) {
	var req decisionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	ctx := r.Context()
	v, err := s.validate(ctx, req.authRequest)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	target, _ := url.Parse(req.RedirectURI)
	params := target.Query()
	if req.State != "" {
		params.Set("state", req.State)
	}
	params.Set("iss", s.base)

	if !req.Approve {
		params.Set("error", "access_denied")
		target.RawQuery = params.Encode()
		writeJSON(w, http.StatusOK, map[string]string{"redirectUrl": target.String()})
		return
	}

	userIDStr, _ := auth.UserID(ctx)
	var userID pgtype.UUID
	if err := userID.Scan(userIDStr); err != nil {
		writeError(w, http.StatusUnauthorized, "access_denied", "login required")
		return
	}
	scopes := []string{ScopeRead}
	if req.Write && v.canWrite {
		scopes = append(scopes, ScopeWrite)
	}
	code, hash := newCode()
	if err := s.q.CreateOAuthCode(ctx, db.CreateOAuthCodeParams{
		CodeHash:      hash,
		ClientID:      v.client.ID,
		UserID:        userID,
		RedirectUri:   req.RedirectURI,
		CodeChallenge: req.CodeChallenge,
		Scopes:        scopes,
		ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(codeTTL), Valid: true},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "could not issue code")
		return
	}
	params.Set("code", code)
	target.RawQuery = params.Encode()
	writeJSON(w, http.StatusOK, map[string]string{"redirectUrl": target.String()})
}

// ---- token endpoint ----

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid form body")
		return
	}
	clientID := r.PostForm.Get("client_id")
	if u, _, ok := r.BasicAuth(); ok && clientID == "" {
		clientID = u
	}
	var cid pgtype.UUID
	if err := cid.Scan(clientID); err != nil || clientID == "" {
		writeError(w, http.StatusUnauthorized, "invalid_client", "unknown client")
		return
	}
	if _, err := s.q.GetOAuthClient(r.Context(), cid); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_client", "unknown client")
		return
	}

	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.exchangeCode(w, r, cid)
	case "refresh_token":
		s.refresh(w, r, cid)
	default:
		writeError(w, http.StatusBadRequest, "unsupported_grant_type", "use authorization_code or refresh_token")
	}
}

func (s *Server) exchangeCode(w http.ResponseWriter, r *http.Request, clientID pgtype.UUID) {
	ctx := r.Context()
	code, err := s.q.ConsumeOAuthCode(ctx, auth.HashToken(r.PostForm.Get("code")))
	if err != nil || !code.ExpiresAt.Valid || code.ExpiresAt.Time.Before(time.Now()) || code.ClientID != clientID {
		writeError(w, http.StatusBadRequest, "invalid_grant", "invalid or expired code")
		return
	}
	if r.PostForm.Get("redirect_uri") != code.RedirectUri {
		writeError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri does not match the authorization request")
		return
	}
	verifier := r.PostForm.Get("code_verifier")
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	if len(verifier) < 43 || len(verifier) > 128 ||
		subtle.ConstantTimeCompare([]byte(computed), []byte(code.CodeChallenge)) != 1 {
		writeError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}

	var resp tokenResponse
	err = s.inTx(ctx, func(q *db.Queries) error {
		// One connection per user and client: reconnecting replaces the old one.
		if err := q.DeleteOAuthGrantsForClientUser(ctx, db.DeleteOAuthGrantsForClientUserParams{ClientID: code.ClientID, UserID: code.UserID}); err != nil {
			return err
		}
		grant, err := q.CreateOAuthGrant(ctx, db.CreateOAuthGrantParams{ClientID: code.ClientID, UserID: code.UserID, Scopes: code.Scopes})
		if err != nil {
			return err
		}
		resp, err = issueTokens(ctx, q, grant.ID, code.Scopes)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "could not issue tokens")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request, clientID pgtype.UUID) {
	ctx := r.Context()
	t, err := s.q.GetOAuthTokenByHash(ctx, auth.HashToken(r.PostForm.Get("refresh_token")))
	if err != nil || t.Kind != "refresh" || t.ClientID != clientID || !t.ExpiresAt.Valid || t.ExpiresAt.Time.Before(time.Now()) {
		writeError(w, http.StatusBadRequest, "invalid_grant", "invalid or expired refresh token")
		return
	}
	scopes := t.Scopes
	// A client may ask for a narrower scope, never a wider one.
	if req := strings.Fields(r.PostForm.Get("scope")); len(req) > 0 {
		narrowed := []string{}
		for _, sc := range scopes {
			if slices.Contains(req, sc) {
				narrowed = append(narrowed, sc)
			}
		}
		if len(narrowed) == 0 {
			writeError(w, http.StatusBadRequest, "invalid_scope", "scope exceeds the original grant")
			return
		}
		scopes = narrowed
	}
	var resp tokenResponse
	err = s.inTx(ctx, func(q *db.Queries) error {
		// Rotate: the used refresh token is single-use.
		if err := q.DeleteOAuthToken(ctx, t.TokenID); err != nil {
			return err
		}
		if err := q.DeleteExpiredOAuthTokens(ctx); err != nil {
			return err
		}
		resp, err = issueTokens(ctx, q, t.GrantID, scopes)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "could not issue tokens")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func issueTokens(ctx context.Context, q *db.Queries, grantID pgtype.UUID, scopes []string) (tokenResponse, error) {
	access, accessHash := auth.GenerateOAuthToken(auth.AccessTokenPrefix)
	refresh, refreshHash := auth.GenerateOAuthToken(auth.RefreshTokenPrefix)
	now := time.Now()
	if err := q.CreateOAuthToken(ctx, db.CreateOAuthTokenParams{
		GrantID: grantID, Kind: "access", TokenHash: accessHash,
		ExpiresAt: pgtype.Timestamptz{Time: now.Add(accessTokenTTL), Valid: true},
	}); err != nil {
		return tokenResponse{}, err
	}
	if err := q.CreateOAuthToken(ctx, db.CreateOAuthTokenParams{
		GrantID: grantID, Kind: "refresh", TokenHash: refreshHash,
		ExpiresAt: pgtype.Timestamptz{Time: now.Add(refreshTokenTTL), Valid: true},
	}); err != nil {
		return tokenResponse{}, err
	}
	return tokenResponse{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: int(accessTokenTTL.Seconds()),
		RefreshToken: refresh, Scope: strings.Join(scopes, " "),
	}, nil
}

// revoke implements RFC 7009: revoking either token of a connection ends the
// whole connection. Unknown tokens are not an error.
func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err == nil {
		if t, err := s.q.GetOAuthTokenByHash(r.Context(), auth.HashToken(r.PostForm.Get("token"))); err == nil {
			_ = s.q.DeleteOAuthGrantByID(r.Context(), t.GrantID)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) inTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// newCode returns an unprefixed random value (used for authorization
// codes) and its stored hash.
func newCode() (plaintext, hash string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	plaintext = base64.RawURLEncoding.EncodeToString(b)
	return plaintext, auth.HashToken(plaintext)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, desc string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": desc})
}
