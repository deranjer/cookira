// Package api wires up HTTP routes: the huma-generated OpenAPI layer under
// /api/v1 (used by the web UI and external clients), the image endpoints,
// and the MCP server at /mcp.
package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/db"
	"github.com/deranjer/cookira/internal/mcpserver"
	"github.com/deranjer/cookira/internal/oauth"
	"github.com/deranjer/cookira/internal/recipes"
	"github.com/deranjer/cookira/internal/version"
)

type Server struct {
	Router    *chi.Mux
	pool      *pgxpool.Pool
	q         *db.Queries
	mgr       *auth.Manager
	limiter   *auth.LoginLimiter
	publicURL string
	svc       *recipes.Service
	oauth     *oauth.Server
	humaAPI   huma.API
}

// New builds the HTTP server. jwtSecret signs login access tokens.
func New(pool *pgxpool.Pool, svc *recipes.Service, publicURL string, jwtSecret []byte) *Server {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	humaCfg := huma.DefaultConfig("Cookira API", version.Version)
	humaCfg.Servers = []*huma.Server{{URL: "/api/v1"}}
	humaAPI := humachi.New(r, humaCfg)

	q := db.New(pool)
	mgr := auth.NewManager(q, jwtSecret)
	s := &Server{Router: r, pool: pool, q: q, mgr: mgr, limiter: auth.NewLoginLimiter(10, 15*time.Minute), publicURL: publicURL, svc: svc, oauth: oauth.New(pool, mgr, publicURL), humaAPI: humaAPI}

	huma.Register(humaAPI, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/api/v1/health",
		Summary:     "Health check",
		Tags:        []string{"System"},
	}, func(ctx context.Context, input *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = "ok"
		out.Body.Version = version.Version
		out.Body.DBConnected = pool.Ping(ctx) == nil
		return out, nil
	})

	s.registerSetupRoutes()
	s.registerAuthRoutes()
	s.registerUserRoutes()
	s.registerInviteRoutes()
	s.registerRecipeRoutes()
	s.registerShareRoutes()
	s.registerPublicRoutes()
	s.registerImageRoutes(r)
	s.oauth.Mount(r)
	s.registerOAuthConnectionRoutes()
	s.registerMCPRoute(r)

	return s
}

// protected returns the huma middleware chain that guards operations
// behind a person's login.
func (s *Server) protected() huma.Middlewares {
	return huma.Middlewares{requireAuth(s.humaAPI, s.mgr)}
}

// registerMCPRoute mounts the MCP server at /mcp. It accepts only OAuth access
// tokens (agents connect through the OAuth consent flow); an unauthenticated
// request gets a 401 pointing at the OAuth resource metadata. Stateless
// because every request is independently authenticated.
func (s *Server) registerMCPRoute(r *chi.Mux) {
	mcpSrv := mcpserver.New(s.svc)
	handler := mcpsdk.NewStreamableHTTPHandler(
		func(*http.Request) *mcpsdk.Server { return mcpSrv },
		&mcpsdk.StreamableHTTPOptions{Stateless: true},
	)
	r.With(auth.RequireOAuth(s.mgr, s.oauth.ResourceMetadataURL())).Handle("/mcp", handler)
}

type healthBody struct {
	Status      string `json:"status" example:"ok"`
	Version     string `json:"version"`
	DBConnected bool   `json:"dbConnected"`
}

type healthOutput struct {
	Body healthBody
}

type statusBody struct {
	Status string `json:"status"`
}

type statusOutput struct {
	Body statusBody
}

func okOutput() *statusOutput {
	return &statusOutput{Body: statusBody{Status: "ok"}}
}

// mapErr converts domain errors to HTTP problem responses.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, recipes.ErrNotFound):
		return huma.Error404NotFound("not found")
	case errors.Is(err, recipes.ErrInvalid):
		return huma.Error422UnprocessableEntity(err.Error())
	default:
		return err
	}
}

func mustUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}

func ts(t pgtype.Timestamptz) string {
	return t.Time.Format("2006-01-02T15:04:05Z07:00")
}

func nullableTimestamp(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := ts(t)
	return &s
}
